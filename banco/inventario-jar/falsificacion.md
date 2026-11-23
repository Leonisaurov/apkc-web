## Falsificación del contador

- Jar vacío: 0 clases, 0 fallos.
- Fixture sano: 0 límites.
- Fixture con el primer opcode cambiado a `0xca`: 1 límites, el primero en `Lfixture/Sano;.()V pc 0 · byte 0xca`.
- Fixture restaurado: 0 límites.

Veredicto del contador: PASA

## Reconstrucción por huella

- `--apkc` externo: se usa sin reconstruir.
- `APKC=...` externo: se usa sin reconstruir.
- Sin cambios: no reconstruye.
- Con un `.go` cambiado: reconstruye.
- Al restaurar el `.go` original: reconstruye una vez.
- Después de restaurar, sin cambios: no reconstruye.
- `git status --porcelain apkc/` vacío durante toda la comprobación: la sonda no escribe en el árbol versionado.

Veredicto de la reconstrucción: PASA

## Cuarto caso: informe reproducible

- Dos ejecuciones seguidas del mismo jar: `informe.md` idéntico (sha256 `0140ab3b9ee9a2b0…`).
- `casos.json` idéntico (sha256 `ca3d163bab055381…`).
- El oráculo usa `trabajo-95e423499042/d8`, derivado del jar, no un directorio aleatorio.

Veredicto de la reproducibilidad: PASA
