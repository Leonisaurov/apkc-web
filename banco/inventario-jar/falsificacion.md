## Falsificación del contador

- Jar vacío: 0 clases, 0 fallos.
- Fixture sano: 0 límites.
- Fixture con el primer opcode cambiado a `0xca`: 1 límites, el primero en `Lfixture/Sano;.()V pc 0 · byte 0xca`.
- Fixture restaurado: 0 límites.

Veredicto: PASA
