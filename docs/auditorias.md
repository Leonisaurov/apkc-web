# Auditorías de la vista previa, la compilación y la entrega

**Fecha:** 2026-11-22. **Qué es:** consolidación de once subagentes (cuatro auditorías y sus
críticos, más el builder del manifiesto). **Qué no es:** una medición nueva. Los `§` de
`docs/evidencia.md` siguen siendo el único sitio de las mediciones (regla 1); aquí se guardan los
hallazgos de lectura de código y su estado para que sobrevivan a una compactación del historial.
Cada veredicto es `CONFIRMADO`, `INCIERTO` o `FALSO` según el `critico` que lo atacó. Nada se da por
hecho por analogía (regla 11).

## 1. Hallazgos CONFIRMADOS

### 1.1 Integridad de datos del editor

- **F1, carrera de pestañas. CERRADA en `e121270` (submódulo; puntero `6243d9d`).** La causa era
  que `activarDocumento()` fijaba `DOCUMENTOS.activa` y `estadoPestana.ruta` antes de leer OPFS y
  de `EDITOR.poner()`, y `guardar()` no tenía guardia de carga. Ahora `cargandoDocumento` se activa
  con `try/finally` y `guardar()` se bloquea antes del primer `await`, así que no puede escribir el
  buffer del fichero anterior en el nuevo. La prueba F1 de `prueba.js` lo cubre; romper el
  `if (cargandoDocumento)` la hizo fallar y restaurarlo la devolvió a verde. La carrera real con
  DOM y worker queda para el gate con navegador.
- **F2. CERRADO en `e121270`.** `guardarAjustes` aplica sobre el buffer del manifiesto, no sobre una
  lectura de OPFS que pisaba los cambios sin guardar.
- **F4.** `importarProyecto` (`pestana.js:509`) ignora `borradoDelDisco` (`fs-fragmento.js:546`):
  si otra pestaña tiene un handle abierto, el borrado falla y el zip se mezcla con el árbol viejo.
  Tampoco hay rollback si se agota la cuota a mitad de la escritura.
- **TOCTOU.** La revisión se lee y se confirma fuera de `conBloqueo` (`pestana.js:1707-1719`).
- **Escrituras sin `conBloqueo`:** jar (`:2476`, `:2535`), deps (`deps.js:130,139`) e imagen del
  drawable (`previa-panel.js:2096`). La generación de clave sí pasa por lock (`:2237`).
- **F5 (INCIERTO).** Teclear durante la ventana de carga marca sucio el doc nuevo con `texto=null`
  y la lectura posterior descarta lo tecleado. No se cerró con `e121270`; sigue `INCIERTO`.
- **Frontera de gate.** La carrera real con DOM y worker (F1 y F5) no se puede reproducir en
  `prueba.js`; queda para el gate con navegador, igual que los dos puntos `INCIERTO` de la
  interfaz.

### 1.2 Compilación y panel de resultado

- **C1. CERRADO en `e121270`.** Cuando falla `guardarParaCompilar`, el jar o el keystore, el panel
  muestra `no compilé` y deja la línea en consola, en vez de conservar el `0` anterior.
- **C2. CERRADO en `e121270`.** El `verify` interno ya no pisa `#ultimo-codigo`
  (`{recordar:false}`); el panel distingue el código del build del de la verificación.
- **C3. CERRADO en `e121270`.** `Ctrl+Enter` guarda los sucios y repite el build por
  `compilarDebug`/`compilarDelTodo`, sin reutilizar un argv de un build fallido.
- **C4. CERRADO en `e121270`.** `ULTIMO.argv = redactarArgv(argv)` y `ULTIMO.secreto`, con redacción
  también de los valores que empiezan por `-`; la contraseña de release no queda en el argv crudo.
- **C5.** **I3 no existe en el código de este árbol.** `grep -rn 'clasesAlDia\|classRosterSHA' apkc/`
  no da resultados; solo aparecen en `docs/plan-incremental-web.md:248,250,256`.
  `apkc/compilar.go:152` dice «No hay incremental aqui…»; `escribirClases` (`compilar.go:87-98`)
  reescribe todos los `.class`, `compSource` queda siempre cierto (`build.go:300,340,365`) y un
  build inactivo recompila, re-dexa y reempaqueta. `--inc` solo ahorra el enlace de recursos (I1/I2).
- **C6 (INCIERTO):** `saveTable()` se llama antes del empaquetado y `LogF` no volcaría el estado;
  hoy no es alcanzable porque `R.java` fuerza la compilación.

### 1.3 Emulador / vista previa

- **3.1. CORREGIDO en `def0b85` (§206).** Una actividad abierta con `startActivity` que no llama a
  `setContentView` devuelve error y `estabiliza` avisa y reanuda la anterior, en vez de caer al
  layout `main` de la lanzadora (`def0b85`: `previewinterprete.go:777`; antes `:763`;
  `preview.go:314,336`). El crítico validó el arreglo y dio **APROBAR**; queda una pregunta
  `INCIERTA` (abajo).
- **3.2. CORREGIDO en `def0b85` (§206).** El `setOnClickListener` del `onCreate` gana al
  `android:onClick` del XML, como en Android: el enlace declarativo se salta si ya hay un oyente
  programático (`def0b85`: `previewinterprete.go:990`). `pulsa` no se invirtió: sigue valiendo el
  contrato «último registrado gana» de `:925-927`.
- **3.3. CORREGIDO por `81d6acf`.** `getText()` dentro de `onCreate` devuelve el texto actual. No se
  movió `preparaVistasDelLayout` antes de `onCreate`, porque depende de `m.layoutID`. No se repitió
  en E1.1.
- **3.4. CORREGIDO en `def0b85` (§206).** El clic 33 no congela (Go avisa y hace `break`,
  `def0b85`: `previewinterprete.go:695-696`) y ahora tiene prueba
  (`TestElPreviewClicNumeroTreintaYTresNoSeEntrega`). Se añadió el tope simétrico de 32 pantallas
  (`pantallasMaximas`, `def0b85`: `:47-50`, corte en `:639`): una navegación circular que antes
  encadenaba pantallas sin fin ahora avisa y deja delante la última que arrancó. Matiz del crítico:
  los tests aseveran contra las constantes, no contra el literal 32. Las citas de línea son las de
  `def0b85`.
- **3.5.** `try`/`catch` en `onCreate` mata la ejecución entera (`:1044-1046`, `:614-622`).
  Limitación del MVP, sin arreglo trivial.
- **3.6.** `RelativeLayout` y los `*Layout` no lineales se apilan en el origen
  (`preview.go:1369-1373`), ignorando `layout_gravity` que `colocarFrame` sí aplica (`:1448-1476`).
- **D7.** El gate no ejerce `startActivity`: `e2e.sh:190-196` es Contador con un clic y
  `prueba-pestana.js:1644-1648` es una sola actividad. La prueba de navegación en pestaña que E1
  invoca **no existe**.
- **Veredicto E1:** correcta en intérprete y CLI (determinista, estado conservado), incompleta como
  afirmación de la pestaña. La prioridad del crítico **E1.1 antes que E4** se atendió: `def0b85`
  cerró 3.1, 3.2 y 3.4 y el crítico dio **APROBAR** (§206). D7 sigue sin prueba en la pestaña.
- **INCIERTO:** el orden de ciclo de vida de `previewinterprete.go:643` no se ha medido en aparato.
- **INCIERTA (pregunta abierta de E1.1, no bloqueante):** en un redirect sin layout que vuelve al
  frente porque el destino llama a `finish()`, `layoutID==0` podría caer al layout `main`
  (`def0b85`: `previewinterprete.go:777`; `preview.go:314,336`). No se ejecutó el probe de tres
  actividades que lo decidiría, así que no se escribe como cierre.

### 1.4 Entrega

- La copia servida venía del commit viejo del submódulo (`259b475`) con `sw.js` pegado a mano
  (`apkc-studio-local-fixapk-20261122`): si Brave ya registró esa versión, el SW no se reactiva y
  sirve caché antigua. Es la causa del síntoma del usuario, no el manifiesto corrupto.
- `deps-index.json` no existe en el repo ni en local ni en el sitio publicado: el botón «Resolver»
  responde 404. `deps-maven.yml` solo sube un artefacto.
- El job `publicar` de `gate.yml:270-282` solo hace `zip` + `gh release create`; no despliega.
- `docs/plan.md:69` dice del jar «escrito y sin ejecutar», mientras
  `docs/plan-android-jar-propio.md:5-7,244-279` da J4 cerrada con el run `45062792689`.
- **Corrección:** el 404 de `/android-jar` en Surge **no** prueba J5; hay selector de fichero y el
  endpoint es del servidor local.

### 1.5 Regresión del parche ya integrado

El crítico de `70433f1` demostró que `pareceManifiesto` (`pestana.js:1692-1695`) rechaza manifiestos
que apkc acepta: `<!--x--><manifest…>`, `<?xml …?><!--c--><manifest…>` y `<!DOCTYPE manifest>…`.
`GO111MODULE=off go run …/xmlcheck.go` dio `err=<nil>` para los tres. El builder está corrigiéndolo
en su worktree con tres casos nuevos en `prueba.js`.

## 2. Hipótesis descartadas o corregidas

- «El daño del manifiesto fue solo la sonda CDP»: correcto sobre la causa observada, **incompleto**
  porque F1 abre el mismo camino a un usuario normal (§1.1).
- «Crear proyecto borra sin confirmar» (F3): **FALSO**; `vistas.js:93-99` comprueba sucios y llama a
  `confirmarDescartes()`.
- «El clic 33 congela el emulador»: **FALSO**; Go avisa y descarta (§1.3).
- «El 404 de `/android-jar` prueba J5 pendiente»: **FALSO** (§1.4).
- «I3 hecha en parte con clases por hash»: el plan documenta código ausente (§1.2).

## 3. Pendiente, en el orden propuesto

1. El builder: corregir el falso positivo de `pareceManifiesto` y aceptar zips de GitHub con una
   carpeta raíz única; después integrar + gate + `critico`.
2. **F1 cerrada** en `e121270`: `cargandoDocumento` con `try/finally` y `guardar()` bloqueado antes
   del primer `await`; la prueba de F1 en `prueba.js` se falsificó y quedó verde. La carrera real
   con DOM y worker queda para el gate con navegador.
3. **C1-C4 cerrados** en `e121270`: `no compilé` cuando falla guardado/jar/keystore, `verify` sin
   pisar `#ultimo-codigo`, `Ctrl+Enter` que guarda sucios y repite build, y `ULTIMO.argv` redactado
   con `ULTIMO.secreto`.
4. Regenerar el sitio servido con `tools/entorno.sh sitio` y subir la versión del SW para que el
   navegador del usuario vea el código actual; publicar solo si el usuario lo pide.
5. E1.1 **cerrada** en `def0b85`: arreglo 3.1, precedencia de oyentes (3.2), siembra de textos
   (3.3, ya hecha por `81d6acf`) y topes de clics y pantallas (3.4). Evidencia y comandos en §206;
   crítico `ses_df9f4b884ffepDlg2y2VQsIBYZ` en **APROBAR**. Queda de ella la prueba de navegación
   en pestaña (D7), que sigue pendiente, y la pregunta `INCIERTA` del redirect sin layout, sin probe.
   Después, E4.
6. F4: abortar el import si `borradoDelDisco === false`. Decidir `deps-index.json` y corregir
   `docs/plan.md:69`.

## 4. Estado verificado

- **Editor:** `e121270` en el submódulo (cherry-pick de `e600a66` del worktree `editor-auditoria`),
  puntero `6243d9d` en la raíz. `node --check` de `pestana.js`, `ajustes.js` y `prueba.js` sin
  salida; `node prueba.js` → `prueba: todo en orden`, rc=0. Falsificación de F1: romper
  `if (cargandoDocumento)` hizo fallar la prueba y restaurarlo la devolvió a verde.
- **Interfaz del emulador:** el crítico aprobó `fe4771e`/`0b6dc83` sin hallazgos confirmados; dos
  puntos quedan `INCIERTO` para el gate con navegador: que `apkc preview /sdk/ultimo.apk` acepte la
  ruta OPFS y que el arrastre del inspector no desalinee clics tras apagar y encender.
- Red de seguridad del manifiesto integrada: submódulo `70433f1`, puntero `7f2fae6`; gate sobre el
  árbol integrado con `PREFIX`/`GOTMPDIR` en disco: `pestaña: 223 comprobaciones`, `e2e: todo en orden`.
- Reglas de tmpfs y limpieza: commits `7b61cf7`, `6a24066`, `6ad5aac`, todos sobre el `AGENTS.md`
  local del repo.
- `/tmp` quedó en 870 M / 3,5 G; la caché de trabajo en 16 M; los 6,6 G viejos están en la papelera
  a la espera de vaciarse. El servidor sigue en `http://127.0.0.1:8790/` (HTTP 200).
