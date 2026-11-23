# Plan de apps de ejemplo para probar el emulador y el compilador

Cinco proyectos Android mínimos pero con sustancia, en `banco/apps-ejemplo/`, pensados como
objetivos de prueba del emulador y del compilador Go de `apkc`. Los cinco **compilan** con el camino
todo-Go (`--linker go --compiler go`), se **instalan** en Droidloom (`10.177.0.2:5555`) y **arrancan**
su actividad. Lo que no está medido se dice en la columna o en la sección final; no se da por hecho.

Cada app lleva sus fuentes Java y sus recursos, y ninguna versiona binarios: `banco/apps-ejemplo/.gitignore`
ignora `*/build/`, `*.apk` y `*.idsig`.

## Cómo se construyen y se prueban

```sh
# las cinco, una detrás de otra; deja los apks en $HOME/.cache/apkc-web/apps-ejemplo
sh banco/apps-ejemplo/construir.sh

# instala y arranca las cinco en Droidloom (install -r, conserva preferencias)
sh banco/apps-ejemplo/probar.sh
```

El comando individual de una app, desde la raíz del repo, es el mismo que ejecuta el guion:

```sh
CACHE=$HOME/.cache/apkc-web/apps-ejemplo
JAR=$(sh tools/entorno.sh jar)
cd banco/apps-ejemplo/<app> && "$CACHE/apkc" build --linker go --compiler go \
  --android-jar "$JAR" --sdk-version 34 --codename 14 --keystore "$CACHE/debug.keystore"
```

El keystore es de usar y tirar (`apkc debug-keystore`), no toca `~/.android` y no se versiona.

## Las cinco apps

| App | Paquete | Compila | Instala | Arranca | Emulador fiel | Comando que lo comprueba | Frontera conocida |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `lienzo` | `com.ejemplo.lienzo` | sí | sí | sí (pid vivo) | **no medido** | `sh banco/apps-ejemplo/construir.sh` y `adb -s 10.177.0.2:5555 shell am start -n com.ejemplo.lienzo/.MainActivity` | La `onDraw` se parte en métodos de una figura porque el emisor no reutiliza registros; el patrón es fijo y no mide comparación de píxeles |
| `notas` | `com.ejemplo.notas` | sí | sí | sí (pid vivo) | **no medido** | igual con `com.ejemplo.notas` | La persistencia entre arranques no se ha ejercitado a mano; `EditText.getText()` se declara como `TextView` por ambigüedad de sobrecargas |
| `chatbt` | `com.ejemplo.chatbt` | sí | sí | sí (pid vivo) | **no medido** | igual con `com.ejemplo.chatbt` | Sin `BluetoothSocket`: no hay conexión ni envío real; el `ACTION_FOUND` no se recoge (ver "defectos que destapa") |
| `tareas` | `com.ejemplo.tareas` | sí | sí | sí (pid vivo) | **no medido** | igual con `com.ejemplo.tareas` | La persistencia entre arranques no se ha ejercitado a mano; las categorías son botones, no `Spinner` |
| `cronometro` | `com.ejemplo.cronometro` | sí | sí | sí (pid vivo) | **no medido** | igual con `com.ejemplo.cronometro` | No se ha medido la precisión del `postDelayed` ni la deriva del reloj en el emulador |

«Emulador fiel» significa que se haya comparado lo pintado con una referencia (captura o medida de
píxeles). Aquí solo se comprobó que la actividad se muestra y el proceso sigue vivo; no se afirma
fidelidad de dibujo.

## Prioridad sugerida, una a una

1. **`lienzo`** — primero porque es la sonda más dura del emulador (Canvas, Paint, `invalidate` y
   coordenadas flotantes) y la que ya destapó el defecto más caro del compilador: el reparto de
   registros no se reutiliza y una `onDraw` normal se queda sin forma corta. Arreglar eso beneficia
   a las otras cuatro.
2. **`notas`** — segundo porque cubre el ciclo completo de recursos (layout XML, `ArrayAdapter`,
   `SharedPreferences`) sin fronteras externas; sirve para validar el inflado de layouts y la
   persistencia con pasos manuales cortos.
3. **`cronometro`** — tercero porque añade el bucle de UI (`Handler.postDelayed`, `Runnable`,
   `removeCallbacks`) y es fácil de observar: iniciar, parar y vuelta dejan marcas visibles.
4. **`tareas`** — cuarto porque repite adaptador y preferencias con más estado (tres listas
   paralelas) y categorías; su valor es la persistencia y el redibujado, no una API nueva.
5. **`chatbt`** — último porque su parte interesante (socket y recepción de descubrimiento) choca
   con dos fronteras del subconjunto; primero conviene cerrar las anteriores y después decidir si se
   amplía el emisor o se deja la app como está.

## Defectos del compilador que destapa cada una

Las frases siguientes se vieron como rechazo literal de `apkc build --linker go --compiler go`
mientras se escribían estas apps, y el fuente quedó adaptado para compilar. **No se conserva el
comando ni el `§` que las reproduzca**: por regla 1 del `AGENTS.md` son **hipótesis de trabajo**, no
mediciones registradas. Para convertirlas en evidencia hay que restaurar el fuente que las provocó,
repetir el build y anotar el error literal con su comando en `docs/evidencia.md`. El crítico de este
frente marcó precisamente este punto como pregunta abierta.

| App | Defecto o límite | Síntoma literal (resumen) |
| --- | --- | --- |
| `lienzo` | **El reparto de registros no se reutiliza.** Cada expresión reserva un hueco para todo el método y con más de quince no cabe la forma corta. | `register v46 does not fit a four-bit format (the /range or /from16 form is not measured here)` |
| `lienzo` | **`Math.cos`/`Math.sin` con argumento `float`.** El ensanchamiento del argumento a `double` no se contabiliza en la pila. | `quedan -2 valores en la pila al cerrar el metodo` (se corrigió pasando un `double`) |
| `lienzo` | **Producto de un entero por un flotante cuando un operando es una expresión compuesta.** | `* con un F y un operando cuyo tipo no se puede saber antes de escribirlo` |
| `notas` | **Tipos anidados en declaraciones e imports.** `LinearLayout.LayoutParams` y `SharedPreferences.Editor` no se resuelven. | `el tipo LayoutParams no se resuelve` / `se esperaba un nombre de miembro y hay "<"` (genéricos en campo) |
| `notas` | **`EditText.getText()` es ambiguo** entre la sobrecarga de `TextView` y la de `EditText`. | `getText tiene 2 sobrecargas aplicables ... y ninguna es la mas especifica` |
| `notas` | **`android.R` no se resuelve** por nombre cualificado. | `el tipo android no se resuelve` |
| `chatbt` | **Clase anónima que extiende una clase** (no una interface) no está medida. | `una anonima sobre BroadcastReceiver, que no es una interface, no esta medida` |
| `chatbt` | **Asignar una subclase a un campo del supertipo** se rechaza por nombre. | `receptor es Landroid/content/BroadcastReceiver; y se le asigna un Lcom/ejemplo/chatbt/MainActivity$Receptor;` |
| `chatbt` | **Una condición que no es una comparación** (llamada booleana) no está medida. | `una condicion que no sea una comparacion no esta medida` |
| `chatbt` | **`&&` en una condición** no está medido. | `el operador "&&" en una condicion no esta medido` |
| `chatbt` | **Constantes `String` del framework como argumento** (`BluetoothDevice.ACTION_FOUND`, `Manifest.permission.*`) se intentan plegar como entero. | `la constante ... no es un entero: doblarla no esta medida` |
| `chatbt` | **Inicializador de arreglo sin `new`** (`String[] x = { ... }`) no se reconoce. | `"{" no es una expresion que este parser reconozca` |
| `tareas` | **`&&` en una condición** (mismo límite que en `chatbt`). | igual |
| `tareas` | **Presupuesto de registros por método** en `cargar`/`redibujar`/`completar`. | `register v17/v20 does not fit a four-bit format` |
| `cronometro` | **Concatenar un `long`** con una cadena no está medido. | `concatenar una expresion de tipo J: solo se midieron entero, cadena y referencia` |
| `cronometro` | **`putfield` de un campo `boolean`** no está medido; se usó un `int`. | `putfield: a field access of type Z: only int, float, object and wide fields were measured` |
| `cronometro` | **Presupuesto de registros por método** en `formato`; se partió en tres métodos. | `register v24 does not fit a four-bit format` |

Ninguno de estos defectos se corrigió en `apkc/`: el encargo era dejar las apps compilando y los
límites documentados. El arreglo del reparto de registros es el que más desbloquea.

## Qué mide cada app del emulador

| App | Lo que pone a prueba |
| --- | --- |
| `lienzo` | Dibujo por software: `Canvas.drawColor`, `drawCircle`, `drawLine`, `drawText`, `Paint` con estilo y ancho, `invalidate` desde un clic |
| `notas` | Inflado de un layout XML con `LinearLayout`, `TextView`, `Button`, `ListView`; `ArrayAdapter` sobre una raíz `TextView`; `SharedPreferences` con clave por nota |
| `chatbt` | Diálogo de permisos en tiempo de ejecución, `BluetoothAdapter.getDefaultAdapter`, `isEnabled`, `getBondedDevices`, `startDiscovery` |
| `tareas` | Estado en tres listas paralelas, redibujado del adaptador, persistencia con contador y marcas de hecha |
| `cronometro` | `Handler.postDelayed` a 50 ms, `SystemClock.elapsedRealtime`, `removeCallbacks` y actualización de la UI desde el hilo principal |

## No medido todavía

Comandos pendientes, en el orden en que conviene ejecutarlos:

```sh
# 1. Fidelidad de dibujo: captura de lienzo y comparación con una referencia
adb -s 10.177.0.2:5555 exec-out screencap -p > "$HOME/.cache/apkc-web/apps-ejemplo/lienzo.png"

# 2. Persistencia de notas entre dos arranques (guardar, forzar parada, volver a abrir)
adb -s 10.177.0.2:5555 shell am force-stop com.ejemplo.notas
adb -s 10.177.0.2:5555 shell am start -n com.ejemplo.notas/.MainActivity

# 3. Persistencia de tareas y marca de completada tras reiniciar el proceso
adb -s 10.177.0.2:5555 shell am force-stop com.ejemplo.tareas
adb -s 10.177.0.2:5555 shell am start -n com.ejemplo.tareas/.MainActivity

# 4. Deriva del cronómetro: arrancar, esperar un minuto y comparar con el reloj del host
adb -s 10.177.0.2:5555 shell am start -n com.ejemplo.cronometro/.MainActivity

# 5. Bluetooth: estado del adaptador y equipos vinculados en el emulador
adb -s 10.177.0.2:5555 shell dumpsys bluetooth_manager | head -40

# 6. Verificación de los apks con los oráculos ajenos
for a in lienzo notas chatbt tareas cronometro; do
  apksigner verify --verbose "$HOME/.cache/apkc-web/apps-ejemplo/$a.apk"
  zipalign -c -p 4 "$HOME/.cache/apkc-web/apps-ejemplo/$a.apk"
done
```

Tampoco está medido: la interacción por toques reales (los `onClick` se compilan, pero no se pulsaron
en el emulador), el diálogo de permisos de `chatbt` (puede quedar pendiente en pantalla) y el
comportamiento de `startDiscovery` sin receptor. Cada uno tiene su comando arriba o su frontera en la
tabla de defectos.

## Fase H — mejorar las apps y endurecer el emulador con ellas

Autorizada por el usuario el 2026-11-22. Estado: **H1 y H2 en curso**, con un builder en el worktree
`apps-mejorar`; **no se corre ningún gate final** hasta que el usuario lo pida.

| Paso | Objetivo | Comando que lo prueba | Criterio de cierre |
| --- | --- | --- | --- |
| H1 | Dar comportamiento real a las cinco apps: `lienzo` con toques y colores, `notas` con crear/editar/borrar y persistencia, `chatbt` con envío local, `tareas` con filtro y persistencia, `cronometro` con iniciar/pausar/reiniciar. | `sh banco/apps-ejemplo/construir.sh` y `sh banco/apps-ejemplo/probar.sh` | Las cinco siguen compilando con `--linker go --compiler go`, instalando y arrancando en Droidloom (`10.177.0.2:5555`). |
| H2 | Usarlas como sonda: `banco/apps-ejemplo/endurecer.sh` compila, instala, arranca, ejecuta `apkc preview` con una secuencia de `--pulsar`, guarda el JSON del modelo y vuelca `uiautomator dump` del dispositivo. | `sh banco/apps-ejemplo/endurecer.sh` | Informe con diferencias concretas de texto/vistas por app; cada diferencia se registra como aviso, no se oculta. |
| H3 | Convertir cada diferencia en un defecto medido del compilador o del intérprete, con su `§` en `docs/evidencia.md` y su fila en el inventario, y repetir la sonda. | el comando del defecto correspondiente | La diferencia desaparece o queda escrita como frontera con su comando. |

Prioridad dentro de H2/H3: `lienzo` (ya destapó el reparto de registros) → `notas` (inflado y
persistencia) → `cronometro` (bucle de UI y reloj lógico) → `tareas` (estado compuesto) → `chatbt`
(frontera Bluetooth).

Lo que H1 **no** promete: sockets Bluetooth reales, permisos concedidos automáticamente ni
fidelidad de píxeles. Lo que H2 **no** es: un gate; es una sonda reproducible que corre en local y
contra Droidloom, sin chromium ni publicación.
