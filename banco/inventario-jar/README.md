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

El binario de caché se reutiliza solo si su **huella de contenido** coincide con la de este árbol:
SHA-256 sobre los `.go` de `apkc/` sin `_test.go`, `go.mod`/`go.sum` y los ficheros embebidos de
`apkc/template/`. La huella se guarda al lado del binario (`apkc.huella`). Si el código cambió, se
recompila antes de medir; si solo cambió el mtime o se cambió de rama sin cambiar bytes, se
reutiliza. Con `--apkc` o `APKC=...` se usa el binario externo tal cual, sin reconstruir.

`--falsificar` comprueba las dos cosas sobre una **copia** de `apkc/` en el directorio de trabajo
(nunca escribe en el árbol versionado): cambia un `.go` de la copia y exige reconstrucción, restaura
el contenido y exige una última reconstrucción, y una ejecución más sin cambios no debe compilar.
Durante toda la sonda se exige `git status --porcelain apkc/` vacío. El cuarto caso comprueba que
dos ejecuciones seguidas del mismo jar dejan `informe.md` y `casos.json` byte a byte iguales y que
el comando del oráculo usa un directorio derivado de la huella del jar, no uno aleatorio.

## Qué deja

- `informe.md`: tabla de límites con clases y métodos afectados, primer caso con su pc y opcode,
  propuesta de forma DEX y el dex de referencia que escribe `d8` para el mismo método.
- `casos.json`: todos los rechazos con clase, método, pc, opcode y motivo, para que cada agente
  ataque un límite sin volver a correr el jar.
- `falsificacion.md`: el resultado de `--falsificar`.

Los temporales van a `$HOME/.cache/apkc-web/inventario-jar/`, nunca a `/tmp`: esa tmpfs tiene cuota
de usuario y ya ha roto builds de este repo.

## Límite de la medida

Cada clase se dexó sola, así que el conteo de clases es exacto. El de métodos es la primera que
falla dentro de cada clase, porque el ensamblado se detiene ahí: es una cota inferior. Si una clase
tiene dos límites distintos, el informe solo ve el primero.
