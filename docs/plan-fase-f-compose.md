# Plan de Fase F (§363) — runtime Compose, opción (b)

**Decisión del usuario**: invertir en emular Compose; no `compose-sin-modelo`. Estado: plan criticado en modo A y aprobado **con 5 condiciones**, ya incorporadas.

## 1. Objetivo y criterio de hecho
`apkc preview F-Droid.apk` ejecuta composables reales del dex y publica nodos con texto de `stringResource`/`Text`, sin la pantalla falsa `res/A1.xml`. Criterio falsable sin navegador: `layout` empieza por `compose:` y no es `res/A1.xml`; `ejecucion.error == ""`; al menos un aviso `compose:Text:<texto>` cuyo texto aparece en `resources.arsc`; un nodo con ese mismo texto. Comando: `APK_FDROID=/home/leonisauro/Downloads/F-Droid.apk tcr -j 1 -q -- sh -c 'sh tools/materializar-emulador.sh && cd apkc && go test -run TestComposeSetContentPublicaTexto -v .'` (si falta el APK, `t.Fatal`, no `t.Skip`). Textos medidos con `aapt2 dump resources`: `string/app_list_author` → `Apps by %s`, `string/menu_install` → `Install`, `string/installing` → `Installing…`, `string/anti_features_title` → `This app has anti-features`. La cláusula visual de la pestaña no es falsable sin CDP: se difiere a Fase G.

## 2. Evidencia del dex
`MainActivity.onCreate` llama a `androidx.activity.compose.ComponentActivityKt.setContent$default`, al lambda de `ComposableLambdaKt.composableLambdaInstance` y a `org/fdroid/ui/MainKt.Main`, con `Composer.shouldExecute`, `.rememberedValue`, `.changedInstance`, `.updateRememberedValue` y `.skipToGroupEnd`. Conteo medido: `skipToGroupEnd 64`, `shouldExecute 62`, `updateRememberedValue 59`, `rememberedValue 59`, `changedInstance 57`, `changed 51`, `startReplaceGroup 31`, `endReplaceGroup 28`, `startRestartGroup 18`, `endRestartGroup 18`, `useNode`/`startReusableNode`/`getInserting`/`getApplier`/`createNode`/`endNode` 15, `consume 8`, `startDefaults`/`getDefaultsInvalid`/`endDefaults` 2. También `stringResource` y `Text` (classes3), `mutableStateOf`, `rememberSaveable`, `Scaffold` y `NavDisplay`. `composableLambdaInstance`: 284 apariciones en classes3.dis y 56 en classes2.dis, sin variantes — afirmación acotada a esta APK y estos dos desensamblados (`~/.cache/apkc-web/plan-f/`).

## 3. Enganche y estado (condición 1 y 3)
El enganche no se fija por número de línea: se inserta en `invoca`, después de la validación de
registros y antes del comentario `// 1) las clases del propio dex`. En el árbol pre-integración
(HEAD `04456d2`) eso era `:3846`; con B y D integrados el número se desplaza (+21 con B, ~+46 con
B+D), así que la comprobación es semántica:

grep -n 'sobre un objeto nulo\|faltan registros para sus argumentos\|atiendeCompose\|// 1) las clases del propio dex' studio/apkc-emulador/previewinterprete.go

Las cuatro líneas deben aparecer en ese orden, con `atiendeCompose` entre la validación y el paso 1.

Test `TestComposeReceptorNuloSigueDandoError`: un `invoke-interface` a `Composer.shouldExecute` con receptor nulo debe devolver el error y no publicar nodo ni aviso `compose:`.

Estado nuevo en `maquinaDalvik` (`:521-604`, junto a `cola []tareaReloj` en `:602`): campo `composicion *composicionActiva` con `activa bool`, `raices map[*objetoDalvik]bool`, `pila []*objetoDalvik`, `grupos []int`, `recordados map[string]any`, `version int`, `emitidos []string`. No se toca `objetoDalvik` (`:294`).

Algoritmo de pertenencia, sin heurística por nombre ni prefijo: (1) solo `ComponentActivityKt.setContent$default` pone `activa=true` y registra la identidad del `Function2` recibido en `raices`; el `Composer` apunta a esa estructura; (2) en `invoca` (enganche semántico de la §3) un `ComposableLambda` solo se admite si el receptor es `*objetoDalvik` y `raices[receptor]==true`, si no se despacha como invoke ordinario; (3) al admitirlo se hace `push` en `pila`, `Text`/`stringResource` atribuyen el nodo al composable de la cima y guardan el texto en `emitidos`, al volver se hace `pop`; (4) `composableLambdaInstance` con `pila` no vacía registra el objeto devuelto en `raices` (pertenencia transitiva solo dentro de una composición viva); (5) `--recrear` conserva `recordados` en `estadoGuardado` (`:595-596`) y reinicia `raices`. Test `TestComposeNoAceptaUnLambdaFueraDeLaComposicion`: un lambda con `Composer` en su proto pero no registrado no emite nodos ni avisos.

## 4. Pestaña (condición 4)
`grep -rn 'compose:' studio/*.js` da 0 coincidencias; `layoutDelModelo()` (`previa-panel.js:3541-3544`) devuelve `modelo.layout` crudo. Es cambio nuevo, no reutilización. Implementar en `vista-previa.js:774-783`, dentro de `textoDelModelo` (función pura, consumida en `previa-panel.js:38,2718,3496`): si `modelo.layout` empieza por `compose:`, devolver `"Emulador aproximado de Compose " + sufijo + ": " + nodos + " nodos en una pantalla de " + ancho + "×" + alto + " dp."`. `ejecucion.avisos` ya se pinta en `previa-panel.js:193-195,1837`. Prueba nueva en `studio/prueba.js` junto a `:1347-1356` con JSON sintético `{...modeloPrevia, layout: "compose:org/fdroid/MainActivity"}`: exige `/^Emulador aproximado de Compose/` y que no aparezca `res/A1.xml`. Verificar con `node --check studio/vista-previa.js` y `cd studio && node prueba.js`.

## 5. Subfases, abandono y coste (condición 5)
- **F0** `setContent`, desenvoltura del lambda, stubs `Composer`, `stringResource`/`Text` → un nodo con texto real. Abandono: tras dos intentos de desenvoltura, si el test de F0 no produce ni un `compose:Text:` con texto de la lista medida, parar y entregar evidencia.
- **F1** `mutableStateOf`/snapshot y recomposición con `--pulsar`/`--recrear`. `grep -cn 'coroutine\|remember' studio/apkc-emulador/previewinterprete.go` da 0; el mecanismo real es `cola []tareaReloj` (`:602`) y `ejecucion.tareas`. Abandono: si la recomposición no converge con `tareas <= 50` en `TestComposeRecomposicionCambiaElTexto`; sin planificador de coroutines.
- **F2** navegación3 `NavDisplay`/`Navigator`, ruta inicial sin transiciones. Abandono: si no elige la ruta en un solo pase (`TestComposeNavegacionSinTransiciones`).
- **F3** `rememberSaveable` sobre `estadoGuardado` y `MaterialTheme`/`Scaffold` con los colores de E. Abandono: si no sobrevive a `--recrear` (`TestComposeSaveableSobreviveAlRecrear`); fidelidad cromática a E/G.

Coste medido de los frentes ya en marcha (`git -C <worktree>/studio/apkc-emulador diff --stat HEAD`): C 4 ficheros 530+/83−, B 2 ficheros 170+/81−, D 2 ficheros 431+/3−, E 2 ficheros 306+; base C+D+E ≈ 1267 inserciones. F se estima en varios miles de líneas, un orden de magnitud por encima: **estimación, no medición**.

## 6. Reutilización, riesgos, tests y commits
Reutiliza el marco `sin-layout-xml` de C, los 12 stubs de D, el tema/filas de E y el multidex de B: si no están integrados, F solo se falsifica con el APK real. Riesgos: solapamiento con C en `preview.go:786-799`, con D en `invoca:3884`, con E en tema/filas; olvidar `sh tools/materializar-emulador.sh`; romper la guardia `git -C studio rev-parse HEAD:apkc-emulador` = `4117a8f`. Mitigación: enganche semántico en `invoca` tras las validaciones (§3), `atiendeCompose` antes de `intentaStub` y `cmp` del materializador. Tests: `TestComposeSetContentPublicaTexto`, `TestComposeReceptorNuloSigueDandoError`, `TestComposeNoAceptaUnLambdaFueraDeLaComposicion`, `TestComposeRememberConservaElValor`, `TestComposeRecomposicionCambiaElTexto`, `TestComposeNavegacionSinTransiciones`, `TestComposeSaveableSobreviveAlRecrear`, `TestComposeF-DroidSinPantallaFalsa` y la prueba de node. Commits previstos, sin push sin autorización: en el submódulo `feat(preview): ejecuta composables y publica nodos`, `feat(preview): estado y recomposición de Compose`, `feat(preview): navegación3 sin transiciones`, `feat(preview): rememberSaveable y tema Material3`; en `studio/` `test(previa): falsifica la clasificación Compose sin navegador` y `chore(studio): actualiza el puntero del emulador`; en el padre `chore: actualiza el gitlink de studio`.

## 7. Prohibido
No pintar pantalla ni texto inventado; no despachar por prefijo de paquete ni por firma con `Composer`; no reimplementar B/C/D/E; no meter runtime de terceros (regla 4); no tocar gate, CDP, chromium, red ni publicación; no anunciar fidelidad sin Fase G; no dar F por verde sin las condiciones anteriores.
