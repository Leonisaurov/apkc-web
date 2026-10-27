# Plan

Regla del repo: **ninguna fase empieza hasta que la anterior dio un resultado escrito**, y **ningún
cambio entra en `../apkc` porque sí** — solo los que mejoran la herramienta que ya funciona. La
ambición de este repo puede morir en cualquier fase; lo que no puede es arrastrar a apkc consigo.

## Fase 0 — lo nuestro primero (esto NO es de este repo, pero lo manda)

Mientras apkc y su página de fricción no hagan lo que deben, aquí no se toca código. Pendiente
aceptado por el usuario:

1. `LogF("build", string(out))` descarta `err` en cuatro sitios -> la línea sale vacía cuando el
   programa ni arrancó. Sin esto, el navegador (y el teléfono) se quedan sin diagnóstico.
2. `apkc sign` / `apkc verify` como comandos sobre un apk existente: la firma ya es Go, así que es
   **la única pieza de apkc que hoy funcionaría en el navegador**, y mientras viva dentro de `build`
   no hay nada que ejecutar del lado cliente.

## Fase 1 — `javac` en el navegador

**Qué prueba**: que un `.java` de un proyecto de apkc se convierta en `.class` dentro de Chromium,
sin nada instalado.

**Cómo**: bajar la demo de GraalVM (`javac` compilado a wasm con native-image) y, como alternativa,
CheerpJ. Compilar `MainActivity.java` del proyecto que genera la página y comparar los `.class` con
los del `javac` nativo (`javap -c` como oráculo, que en este teléfono `dexdump` no vale).

**Mata la fase**: si la licencia de la opción que funciona no permite usarla así, o si el runtime
pesan tanto que la página no arranca. **Antes de escribir una línea, leer la licencia**: CheerpJ es
comercial con tier gratuito no-comercial.

**Coste**: días. Es integración.

## Fase 2 — `d8` sobre la JVM wasm <- la que decide todo

**Qué prueba**: que el `d8` real de build-tools 34 corra sobre el runtime de la Fase 1 y produzca un
`classes.dex` **idéntico byte a byte** al que produce el `d8` nativo sobre los mismos `.class`.

**Por qué es la decisiva**: si `d8` no corre (nio, concurrencia, reflexión sobre el classpath) o
tarda minutos, no hay compilación en el navegador por mucho Go que escribamos, y el resto de este
plan se convierte en "escribir un dexer en Go", que es otro proyecto de otra escala.

**Mata la fase**: cualquier diferencia de bytes no explicada, o un tiempo que haga el flujo
inusable. Se documenta el número igual: es el resultado.

**Coste**: días, y **no se hace antes de la Fase 1** porque necesita el runtime bajo los pies.

## Fase 3 — recursos sin `aapt2` (el trabajo gordo, y el que queda para siempre)

**Qué prueba**: escribir en Go las cuatro salidas que apkc pide a `aapt2` —`resources.arsc`, AXML del
manifest, AXML de los layouts y `R.java`— para un proyecto con `<TextView>` y `<Button>`, y que el
apk resultante **se instale y muestre la pantalla**.

**Oráculo**: `aapt2 dump` sobre el apk bueno, byte a byte contra el nuestro; y el teléfono, que es el
único que dictamina si el `resources.arsc` es legal.

**Por qué va después de 1 y 2**: porque si el dexer no existe, un compilador de recursos no sirve de
nada. **Por qué no se hace en un día**: apkc ya tiene el músculo de formato binario (`classfile.go`,
`zipwriter.go`, `apksig.go`, `jks.go`) y sabe firmar, pero nadie ha escrito esto en Go y hay que
respetar los tipos de referencia y los config flags.

**Coste**: semanas. Se abre en trozos por formato (primero AXML, luego la tabla) y cada trozo con su
test contra `aapt2 dump`.

## Fase 4 — la costura y la página

Solo si 1-3 vivieron: dar a `os/exec` (seis ficheros: `aab.go`, `apk.go`, `build.go`, `device.go`,
`doctor.go`, `proc_*.go`) una interfaz con dos implementaciones —proceso (hoy) y módulo wasm—, y
montar el hospedador de navegador sobre la API medida en
[evidencia.md](evidencia.md) (24 funciones, `constants`, `path.resolve`, `process.*`).

`shim/navegador-sim.js` es el borrador de ese hospedador: funciona, es en memoria, y le falta OPFS.

## Qué se sabe ya, para no repetirlo

- El bloque de firma y empaquetado de apkc **corre en hospedador de navegador**: 88/88.
- `apkc create` **funciona del lado cliente** (17 nodos, 256 ms).
- `apkc build` **no**: muere en el primer proceso externo, y hoy lo hace en silencio.
