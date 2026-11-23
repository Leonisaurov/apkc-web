# Auditorías de la vista previa, la compilación y la entrega

**Fecha:** 2026-11-22. **Qué es:** consolidación de once subagentes (cuatro auditorías y sus
críticos, más el builder del manifiesto). **Qué no es:** una medición nueva. Los `§` de
`docs/evidencia.md` siguen siendo el único sitio de las mediciones (regla 1); aquí se guardan los
hallazgos de lectura de código y su estado para que sobrevivan a una compactación del historial.
Cada veredicto es `CONFIRMADO`, `INCIERTO` o `FALSO` según el `critico` que lo atacó. Nada se da por
hecho por analogía (regla 11).

## 1. Hallazgos CONFIRMADOS

### 1.1 Integridad de datos del editor

- **F1, carrera de pestañas.** `activarDocumento()` fija `DOCUMENTOS.activa` y `estadoPestana.ruta`
  en `pestana.js:1108-1109`, antes de leer OPFS (`:1121`) y antes de `EDITOR.poner()` (`:1144`).
  `guardar()` toma la ruta de `documentoActivo()` y el contenido de `EDITOR.texto()` (`:1706`,
  `:1716`). El velo de `conCarga` es solo un `<div hidden>` (`consola.js:85-94`) y el `Ctrl+S` global
  no tiene guardia de carga (`pestana.js:3089-3092`); `hospedador.js:264-269` mete `leer` en `enCola`,
  así que la ventana dura todo un build. Un usuario normal puede escribir el buffer del fichero
  anterior en el fichero nuevo. **El veto de manifiesto no cierra esta carrera**: solo cubre
  layout→manifiesto; si el buffer pegado es otro manifiesto válido, pasa.
- **F2.** `guardarAjustes` (`ajustes.js:38-55`) lee el manifiesto de OPFS, lo reescribe y marca
  `doc.sucio=false`, pisando cambios sin guardar del buffer.
- **F4.** `importarProyecto` (`pestana.js:509`) ignora `borradoDelDisco` (`fs-fragmento.js:546`):
  si otra pestaña tiene un handle abierto, el borrado falla y el zip se mezcla con el árbol viejo.
  Tampoco hay rollback si se agota la cuota a mitad de la escritura.
- **TOCTOU.** La revisión se lee y se confirma fuera de `conBloqueo` (`pestana.js:1707-1719`).
- **Escrituras sin `conBloqueo`:** jar (`:2476`, `:2535`), deps (`deps.js:130,139`) e imagen del
  drawable (`previa-panel.js:2096`). La generación de clave sí pasa por lock (`:2237`).
- **F5 (INCIERTO):** teclear durante la ventana de carga marca sucio el doc nuevo con `texto=null`
  y la lectura posterior descarta lo tecleado.

### 1.2 Compilación y panel de resultado

- **C1.** Si `guardarParaCompilar` falla o fallan jar/keystore, `compilarCon` (`pestana.js:2651-2652`)
  sale sin tocar `#ultimo-codigo`: queda el `0` anterior y no hay línea en consola.
- **C2.** Tras build bueno, `#ultimo-codigo` muestra el código de `verify` (`:2674`, `:2731`). El
  botón `#verificar-apk` (`:3038`) puede dejar un `0` en pantalla sobre un apk viejo.
  `prueba-pestana.js:4994` compara contra el verify, no contra el build.
- **C3.** `Ctrl+Enter` (`:3101`) → `ejecutarUltimo` (`:1902`) → `correr(ULTIMO.argv)` se salta
  `guardarParaCompilar` cuando el último argv es un `build` fallido.
- **C4.** La contraseña de release queda en `ULTIMO.argv` (`:1920`); `redactarArgv` (`:1908`) solo
  cubre lo pintado.
- **C5.** **I3 no existe en el código de este árbol.** `grep -rn 'clasesAlDia\|classRosterSHA' apkc/`
  no da resultados; solo aparecen en `docs/plan-incremental-web.md:248,250,256`.
  `apkc/compilar.go:152` dice «No hay incremental aqui…»; `escribirClases` (`compilar.go:87-98`)
  reescribe todos los `.class`, `compSource` queda siempre cierto (`build.go:300,340,365`) y un
  build inactivo recompila, re-dexa y reempaqueta. `--inc` solo ahorra el enlace de recursos (I1/I2).
- **C6 (INCIERTO):** `saveTable()` se llama antes del empaquetado y `LogF` no volcaría el estado;
  hoy no es alcanzable porque `R.java` fuerza la compilación.

### 1.3 Emulador / vista previa

- **3.1.** Una actividad abierta con `startActivity` que no llama a `setContentView` cae al layout
  `main` de la lanzadora sin aviso (`previewinterprete.go:763`; `preview.go:491-496,601-606`). El
  crítico validó el arreglo de 4 líneas: devolver error tras `:763`; `estabiliza` (`:649-651`) ya
  avisa y reanuda la actividad anterior.
- **3.2.** El `android:onClick` del XML gana al `setOnClickListener` del `onCreate`, al revés que
  Android (`:757`, `:763-765`, `:929`, `:971`). **No arreglar invirtiendo `pulsa`**: rompe el
  contrato «último sustituye al anterior» de `:925-927`.
- **3.3.** `getText()` dentro de `onCreate` devuelve `""` (`preview.go:383-390`; siembra en `:763`).
  **No mover `preparaVistasDelLayout` antes de `onCreate`**: depende de `m.layoutID`, que solo fija
  `setContentView` dentro de él.
- **3.4 (corregido).** La pestaña no topa la lista de clics (`previa-panel.js:969`), pero el clic 33
  **no congela**: Go avisa y hace `break` (`previewinterprete.go:684-687`). Falta un tope simétrico.
- **3.5.** `try`/`catch` en `onCreate` mata la ejecución entera (`:1044-1046`, `:614-622`).
  Limitación del MVP, sin arreglo trivial.
- **3.6.** `RelativeLayout` y los `*Layout` no lineales se apilan en el origen
  (`preview.go:1369-1373`), ignorando `layout_gravity` que `colocarFrame` sí aplica (`:1448-1476`).
- **D7.** El gate no ejerce `startActivity`: `e2e.sh:190-196` es Contador con un clic y
  `prueba-pestana.js:1644-1648` es una sola actividad. La prueba de navegación en pestaña que E1
  invoca **no existe**.
- **Veredicto E1:** correcta en intérprete y CLI (determinista, estado conservado), incompleta como
  afirmación. Prioridad del crítico: **E1.1 antes que E4**.
- **INCIERTO:** el orden de ciclo de vida de `previewinterprete.go:643` no se ha medido en aparato.

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
2. Cerrar F1: mover `DOCUMENTOS.activa`/`estadoPestana.ruta` a después de `EDITOR.poner`, o impedir
   `guardar()` mientras la pestaña esté cargando; probar la carrera en `prueba-pestana.js`.
3. Que `#ultimo-codigo` distinga build / verify / «no compilé» (C1, C2) y que Ctrl+Enter guarde
   sucios (C3). Limpiar `--storepass` de `ULTIMO.argv` (C4).
4. Regenerar el sitio servido con `tools/entorno.sh sitio` y subir la versión del SW para que el
   navegador del usuario vea el código actual; publicar solo si el usuario lo pide.
5. E1.1: arreglo 3.1, precedencia de oyentes (3.2), siembra de textos (3.3), tope de clics (3.4) y
   prueba de navegación en pestaña (D7). Después E4.
6. F4: abortar el import si `borradoDelDisco === false`. Decidir `deps-index.json` y corregir
   `docs/plan.md:69`.

## 4. Estado verificado

- Red de seguridad del manifiesto integrada: submódulo `70433f1`, puntero `7f2fae6`; gate sobre el
  árbol integrado con `PREFIX`/`GOTMPDIR` en disco: `pestaña: 223 comprobaciones`, `e2e: todo en orden`.
- Reglas de tmpfs y limpieza: commits `7b61cf7`, `6a24066`, `6ad5aac`, todos sobre el `AGENTS.md`
  local del repo.
- `/tmp` quedó en 870 M / 3,5 G; la caché de trabajo en 16 M; los 6,6 G viejos están en la papelera
  a la espera de vaciarse. El servidor sigue en `http://127.0.0.1:8790/` (HTTP 200).
