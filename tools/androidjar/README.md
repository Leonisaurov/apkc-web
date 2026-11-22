# androidjar

Genera un `android-stub.jar` desde definiciones públicas de AOSP. No abre el
`android.jar` de la SDK de Google ni copia ninguna de sus entradas.

## Entradas (Apache-2.0)

- `frameworks/base/core/api/current.txt`
- `libcore/api/current.txt` (java.*, javax.*, org.*)
- `frameworks/base/core/res/res/values/public-final.xml`
- `frameworks/base/core/res/res/values/attrs.xml`
- `frameworks/base/core/res/res/values/attrs_manifest.xml`

La etiqueta que se usa en CI es `android-14.0.0_r75` (API 34). El workflow
`.github/workflows/android-jar.yml` las descarga, ejecuta el generador y valida
el resultado con `apkc compile` y `apkc build --linker go --compiler go`.

## Uso local

```sh
go -C tools/androidjar run . \
  -in "$TMP/aosp" -out "$TMP/dist" -api 34 \
  -tag android-14.0.0_r75 \
  -notice "$TMP/aosp/NOTICE" -license tools/androidjar/LICENSE
```

`-only` acepta una lista de clases separadas por comas. Con las 15 clases que
usan las plantillas del studio el jar baja a unos 105 KB; sin `-only` se
generan todas las clases públicas (unos 2,7 MB).

## Diseño

- `api.go`: lee el formato de firma 2.0. Resuelve clases anidadas a `Outer$Inner`
  y borra los parámetros de tipo de los métodos a su cota.
- `classfile.go`: escribe un ClassFile con pool, campos, `ConstantValue` y
  métodos sin `Code` (marcados `native`). Los constructores llevan un `Code`
  mínimo para que `apkc` resuelva el `<init>` del padre.
- `arsc.go`: escribe un `resources.arsc` con el paquete `android`, los 23
  nombres de tipo y los ids de `public-final.xml`. Añade ids sintéticos para
  los nombres de enum/flags que no son públicos, porque el enlazador los
  necesita para resolver `gravity="center"` sin escribir el id en el APK.

## Límites

- Solo API pública: no hay `@hide`, `@SystemApi`, `@TestApi` ni recursos de
  producto.
- Las firmas genéricas de clase no se conservan; los descriptores se borran a
  su cota, que es lo que necesita `apkc compile`.
- El `resources.arsc` es mínimo: una configuración por tipo, sin el pool global
  de valores.
