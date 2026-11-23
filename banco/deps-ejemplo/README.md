# deps-ejemplo — una app que compila contra un jar local

Sonda de aceptación del frente de dependencias (Fase J de `docs/plan-deps-red.md`): una app
apkc mínima cuyo `MainActivity` llama a `saludo.Saludo`, una clase que **no está en `src/`**
sino en `jar/saludo.jar`. El jar lo construye el propio guion con el JDK: sin red, sin Maven,
sin `apkc-deps` y sin artefactos versionados.

## Comando

```sh
sh banco/deps-ejemplo/construir.sh
```

Variables: `APKC` (binario; si falta se compila en `$HOME/.cache/apkc-web/deps-fixture`),
`ANDROID_JAR` (por defecto, `tools/entorno.sh jar`), `JAVA_HOME`, `D8`, `DEXDUMP` y
`APKC_WEB_CACHE`. Los artefactos van a `$HOME/.cache/apkc-web/deps-fixture/salida`.

## Qué hace

1. Compila `dep/saludo/Saludo.java` con el JDK y lo empaqueta en `jar/saludo.jar`.
2. **Camino nativo**: `javac -classpath android.jar:jar/saludo.jar` y `d8` para la app y para
   la dependencia, en dex separados.
3. **Camino Go**: `apkc build --linker go --compiler go`; el compilador propio tiene que
   resolver `saludo.Saludo` leyendo `jar/*.jar`, sin levantar `javac` ni `d8`. Si el binario
   anuncia una bandera `--jar`, el guion se la pasa.
4. Extrae los `classes*.dex` del apk y comprueba con `dexdump` que:
   - el dex de la app invoca `Lsaludo/Saludo;.saludo`,
   - el apk empaqueta `Lsaludo/Saludo;` (no solo compila contra ella),
   - y compara el dex de la app de los dos caminos: byte a byte y por lista de clases. Si
     difieren, imprime tamaños, el primer byte distinto y las clases que solo están en un
     lado (`R`/`R$string` las añade el enlazador de recursos de apkc). Es una diferencia de
     emisor y de clases generadas, no de resolución.

`R.java` lo escribe el enlazador en `src/` en cada build (como `aapt2`); está ignorado y el
camino nativo lo excluye para que su dex no dependa de restos de una vuelta anterior.

## Éxito

- Código `0`: los dos caminos compilan, los dos dex invocan el método del jar y el apk Go
  lleva la clase de la dependencia.
- Código `3`: el compilador Go resuelve las clases del jar pero **el dex de la dependencia todavía
  lo escribe d8**, con `the d8 of dependency bucket 0 failed: exec: no command`. Ese rechazo es el
  objetivo del frente, no un fallo del fixture: la sonda queda lista para ponerse verde cuando el
  builder lo levante.
- Código `1`: cualquier otro fallo (se imprime el error literal).

## Estado medido

Con el árbol del 2026-11-22, el camino nativo compila y dexa la app (1136 bytes) y la
dependencia (728 bytes), y el camino Go sale con `rc 3` por el rechazo de arriba. El guion no
oculta ese fallo: lo imprime y lo marca como objetivo pendiente. El mensaje anterior
(`--compiler go no dexa dependencias…`) correspondía a una revisión anterior de `apkc` y ya no
aparece en el árbol actual.
