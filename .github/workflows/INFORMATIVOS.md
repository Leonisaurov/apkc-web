# Contrato de los jobs informativos de `gate.yml`

`webkit`, `safari` y `art` son informativos: tienen `continue-on-error: true` y `publicar` no
depende de ellos. Su valor es lo que cuentan, no si pasan. Para que ese relato sea comprobable,
cada job deja un marcador y el job `verificar-informativos` comprueba el contrato.

## Canal entre jobs

El transporte es `needs.<job>.outputs.marcador`. Cada job informativo declara:

```yaml
outputs:
  marcador: ${{ steps.marcador.outputs.marcador }}
```

y el paso que lo escribe lleva `if: always()`, de modo que el marcador también existe cuando un
paso anterior falla. **No se usa `$GITHUB_STEP_SUMMARY`**: el summary es por job y
`verificar-informativos` no podría leer el de otro job. Los artefactos (`upload-artifact` y
`download-artifact`) siguen transportando el sitio y los logs; no se usan para estos marcadores.

## Marcadores

- `webkit`: `webkit|resultado=<resultado>|comprobaciones=<N>|rc=<rc>`. El resultado y el número
  de comprobaciones se leen del registro de `prueba-webkit.js` y el `rc` se captura del proceso
  real con `PIPESTATUS`.
- `safari`: `safari|motor=<motor>|ejecutable=<ruta>|rc=<rc>`. La ruta y la identidad se leen de la
  salida de `tools/safari-info.mjs`; el `rc` se captura con `PIPESTATUS` del paso `prueba_webkit`
  (el sitio sobre WebKit en macOS). Cuando no hay evidencia de Playwright el motor se declara
  `sin-identidad`, que el verificador rechaza porque exige `playwright-webkit`.
- `art` sin KVM: `art|kvm=no|omitido=si`.
- `art` con KVM y banco correcto: `art|kvm=si|banco=rc0`.
- `art` con KVM y banco fallido: `art|kvm=si|banco=rc<N>|fallo=si`, con el `rc` real de
  `banco/art.sh`. El AVD del corredor se elige con `ADB_SERIAL` (variable del runner); sin esa
  variable `banco/art.sh` sigue en la Blackview MEGA 3 y mantiene su guard de marca/modelo.

`omitido` significa que no hubo entorno (`kvm=no`); `fallo` significa que el comando se ejecutó
y devolvió `rc` distinto de cero. Un `omitido=si` sin `kvm=no` es una omisión sin causa y falla
la verificación. El `rc` nunca se supone: se captura de la ejecución.

## Qué comprueba `verificar-informativos`

Tiene `needs: [webkit, safari, art]` e `if: always()`, **sin** `continue-on-error`. Ya no se conforma
con la presencia de los campos: extrae los valores por regex y exige que todos sean los correctos.

- `webkit`: `resultado=todo-en-orden`, `comprobaciones>=1` y `rc=0`.
- `safari`: `motor=playwright-webkit`, `ejecutable` conocido (ruta de Playwright, no `desconocido`)
  y `rc=0`.
- `art`: `kvm=si`, `banco=rc0` y ausencia de `omitido=si` y de `fallo=si`.

Cualquier otro valor cuenta como problema y falla este job, aunque los tres jobs informativos sigan
siendo `continue-on-error`. `publicar` sigue atado solo a `pesado`, así que este job no puede impedir
la release.

## Estado de esta entrega

El contrato, el canal y el verificador están escritos en el workflow. No se ha corrido el
workflow en GitHub ni se afirma un run verde: la evidencia de ejecución se registrará aparte
cuando haya autorización para correrlo. Lo mismo vale para los 30 SHA de `uses:` pendientes
(`ENDURECER.md`).
