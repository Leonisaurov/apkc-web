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
