# Falsificación del inventario

La salida detallada de `--falsificar` se escribe en la caché, no aquí:

```sh
python3 banco/inventario-jar/inventario.py --falsificar
# detalle: $HOME/.cache/apkc-web/inventario-jar/falsificacion.md
```

No se versiona porque lleva hashes de los fixtures que produce el `javac` local y la ruta del
directorio de trabajo derivado de la huella del jar. Este fichero es el resumen estable.

## Casos que comprueba la sonda

1. **Contador**: un jar vacío da 0 clases y 0 fallos; un fixture sano da 0 límites; el mismo
   fixture con su primer opcode cambiado a `0xca` suma exactamente un caso de «bytecode JVM fuera
   de la tabla»; al restaurar vuelve a 0.
2. **Reconstrucción por huella**: se hace sobre una copia de `apkc/` en el directorio de trabajo,
   nunca sobre el árbol versionado. Sin cambios no recompila; con un `.go` cambiado, sí; al
   restaurar el original, una vez; después, no. Durante toda la sonda se exige
   `git status --porcelain apkc/` vacío.
3. **Informe reproducible**: dos ejecuciones seguidas sobre el mismo jar dejan `informe.md` y
   `casos.json` byte a byte iguales, y el comando del oráculo usa `$CACHE/trabajo-<huella>/d8`, no
   un directorio aleatorio.
4. **No escribe en el árbol**: al terminar, `git status --porcelain` no trae cambios en
   `banco/inventario-jar/` ni en `apkc/`.
5. **Dos corridas concurrentes**: dos procesos a la vez sobre el mismo jar terminan con rc 0 y
   dejan `informe.md` y `casos.json` idénticos. Cada proceso usa un directorio privado
   (`trabajo-<huella>-<aleatorio>`), así que el d8 de uno no borra el del otro.

Cada caso imprime su veredicto; la sonda termina con código distinto de cero si alguno falla.
