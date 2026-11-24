# Evidencia — §117-§139

## 117. Un keystore de release en la pestaña: contraseña, alias y certificado

La página ya tenía un selector de fichero, pero solo pasaba `--keystore`: los valores por defecto
(`--storepass android`, `--keyalias androiddebugkey`) solo abren un keystore de depuración. Un
keystore de release normal tiene otra contraseña y otro alias, y fallaba en uno de los dos sitios:

```sh
P=/data/data/com.termux/files/usr
KEYTOOL=$P/lib/jvm/java-21-openjdk/bin/keytool
"$KEYTOOL" -genkeypair -keystore "$P/tmp/studio-release.p12" -storetype PKCS12 \
  -storepass saludo -keypass saludo -alias lanzable -keyalg RSA -keysize 2048 \
  -validity 3650 -dname "CN=Release de prueba, O=apkc-web, C=ES"
apkc sign --keystore "$P/tmp/studio-release.p12" --out /tmp/x.apk app.apk
# [sign] ... (PKCS#12): the keystore did not open with the given storepass: ... (wrong storepass?)
apkc sign --keystore "$P/tmp/studio-release.p12" --storepass saludo --out /tmp/x.apk app.apk
# [sign] ... holds 1 key(s), the nearest name being "lanzable", and none is androiddebugkey: check --keyalias
```

Ahora `apkc keystore` lista las identidades del fichero con los mismos lectores de PKCS#12 y JKS
del firmante, sin proceso ni JVM, y la página la usa para enseñar el certificado antes de firmar:

```sh
apkc keystore --keystore "$P/tmp/studio-release.p12" --storepass saludo
# keystore-json: {"keys":1,"entries":[{"alias":"lanzable","subject":"CN=Release de prueba,O=apkc-web,C=ES","sha256":"BD:03:7F:CB:4F:D8:79:38:2F:6A:EF:8D:75:0C:62:C2:B6:ED:DA:1B:05:AF:9C:49:78:C4:A9:90:C5:6D:84:F6","notAfter":"2036-11-06"}]}
```

La huella es la misma que imprime `apksigner`, y la salida es **una sola línea con prefijo** para
que la página la parsee sin limpiar los códigos de color de las demás. Una contraseña mala se nombra
como contraseña (`did not open with the given storepass`), no como un fichero corrupto; un alias que
no está se nombra con el más parecido.

La página pide la contraseña en cada firma (no la guarda), elige el alias entre los que declara el
fichero —y lo exige si hay más de uno—, enseña sujeto, huella y validez, y pasa
`--storepass`/`--keyalias` tanto a `apkc sign` como a `build --linker go --compiler go`. «Olvidar»
borra el fichero de OPFS. La prueba de la pestaña genera el keystore de release con `keytool`, lo
mete por la página, firma y apksigner lee el certificado elegido:

```sh
sh -c 'cd studio && APKC_WASM=$P/tmp/apkc-wasm.wasm ... KS_RELEASE=$P/tmp/studio-release.p12 node prueba-pestana.js'
apksigner verify --verbose --print-certs --min-sdk-version 21 "$P/tmp/pestana-apk/pestanera-release.apk"
```

```text
Verified using v1 scheme (JAR signing): true
Verified using v2 scheme (APK Signature Scheme v2): true
Verified using v3 scheme (APK Signature Scheme v3): true
V3.0 Signer: certificate DN: CN=Release de prueba, O=apkc-web, C=ES
V3.0 Signer: certificate SHA-256 digest: bd037fcb4fd879382f6aef8d750c62c2b6edda1b05af9c4978c4a990c56d84f6
```

`studio/e2e.sh` lo exige y lo resume:

```text
pestaña release: apk de 10352 bytes firmado con CN=Release de prueba y verificado por apksigner (v1+v2+v3)
```

Pruebas: `TestInspectarKeystoreListaAliasSujetoYHuella`, `TestInspectarKeystoreNombraLaContrasenaMal`
y `TestInspectarKeystoreLeeJKSYListaSuAlias` en `apkc/keystorelist_test.go`; el caso nuevo de
`studio/prueba-pestana.js` conduce la página (lista, certificado, firma) y `studio/e2e.sh` corre
`apksigner` sobre el apk que sale. Frontera: v4, la rotación v3.1, las claves EC, el `.aab` y el
`minSdk` del proyecto en el firmante v3 quedan como piezas propias (§5.29-§5.35 del plan).

## 118. Un `.aab` en la pestaña: firma JAR v1, sin bloque de apk

Un bundle no es un apk: es un zip con `BundleConfig.pb` y sus módulos, se firma como un JAR y no
lleva bloque de firmas v2/v3 ni pasa por `zipalign`. `apkc sign` lo detecta por la extensión y escribe
solo la v1; el manifiesto va **el primero** porque `JarInputStream` lo busca ahí y `jarsigner` avisa
si no está (`Manifest is missing when reading via JarInputStream`).

```sh
P=/data/data/com.termux/files/usr
W=$P/tmp/aabtest
mkdir -p "$W/base/dex" "$W/base/manifest"
printf 'BundleConfig' > "$W/BundleConfig.pb"
printf 'manifest' > "$W/base/manifest/AndroidManifest.xml"
printf 'dex' > "$W/base/dex/classes.dex"
(cd "$W" && python3 -m zipfile -c in.aab BundleConfig.pb base/dex/classes.dex base/manifest/AndroidManifest.xml)
apkc sign --keystore "$P/tmp/studio-release.p12" --storepass saludo --keyalias lanzable \
  --out "$W/out.aab" "$W/in.aab"
jarsigner -verify "$W/out.aab"
apkc verify "$W/out.aab"
```

```text
[sign] out.aab carries a v1 JAR signature of its own, 3 entries taken over as their bytes already were
jar verified.
[verify] v1 (JAR signing): ok — 3 entradas con su digest y la firma PKCS#7 verificada
[verify] el paquete es un .aab: la firma es JAR v1 y no lleva bloque v2/v3
```

`jarsigner` sigue avisando de que el certificado es auto-firmado y de que no hay timestamp, como con
cualquier clave de prueba; lo que ya no dice es que el manifiesto falte. `apkc verify` reconoce el
`.aab` por la extensión, verifica la v1 con su propio lector y **no** le pide una v2 que el formato no
tiene; `apksigner` no es oráculo aquí porque espera un apk.

La pestaña usa la misma sección de firma: si la ruta de entrada termina en `.aab`, el destino es
`firmado.aab`, el MIME de descarga es `application/octet-stream` y, tras firmar, la propia página
ejecuta `apkc verify` y enseña el veredicto. `studio/e2e.sh` crea un `.aab` mínimo con `python3 -m
zipfile`, lo firma desde la pestaña con el keystore de release y exige:

```text
pestaña .aab: 2340 bytes, v1 JAR verificada por jarsigner y apkc verify, sin bloque v2/v3
```

Pruebas: `TestUnBundleSeFirmaComoJarYManifiestoPrimero` y
`TestSignDistingueElBundlePorLaExtension` en `apkc/bundle_sign_test.go`, más el caso `.aab` de
`studio/prueba-pestana.js`. Construir el bundle (la tabla de recursos en protobuf, `BundleConfig.pb`
y los módulos) es 5.33; aquí solo se firma el que ya existe.

## 119. Firma v4 (`.idsig`): árbol de verity y la firma que va al lado del apk

La v4 no va dentro del apk: es un fichero `.idsig` al lado, el que usa `adb install
--incremental` en Android 11+. Se midió sobre uno que escribe `apksigner
--v4-signing-enabled true` y se reproduce con estos campos, todos little-endian y los
variables con su longitud en u32:

```text
u32 version = 2
bytes hashingInfo:      u32 hashAlgorithm = 1 (SHA-256), u8 log2BlockSize = 12,
                        bytes salt (vacía), bytes rawRootHash (32)
bytes signingInfos:     bytes apkDigest (32), bytes certificate (DER), bytes additionalData,
                        bytes publicKey (SubjectPublicKeyInfo DER), u32 sigAlg = 0x0103,
                        bytes signature
bytes verityTree:       el árbol de fs-verity
```

El `apkDigest` **no** se recalcula sobre el fichero final: es el mismo digest que va dentro
del bloque v3, y por eso el firmante pasa el que ya calculó para v2/v3. La firma es
RSASSA-PKCS1-v1_5/SHA-256 sobre esta carga, que también incluye el tamaño del apk y el
árbol:

```text
u32 tamaño; u64 fileSize; u32 hashAlgorithm; u8 log2BlockSize;
bytes salt; bytes rawRootHash; bytes apkDigest; bytes certificate; bytes additionalData
```

El árbol es el de fs-verity: hojas de 4096 bytes (la última rellena con ceros), cada nivel
partido en bloques de 4096 y relleno hasta un múltiplo de 4096, y el nivel superior el
primero en el fichero. El nivel superior es el primero cuyos hashes caben en 4096 bytes; su
SHA-256 es `rawRootHash`. Para el apk de 12756 bytes de la sonda son cuatro hojas, el nivel
0 (128 bytes de hashes) relleno a 4096, y `rawRootHash = 50081f11…a17c`, el mismo que el
`.idsig` de `apksigner`.

```sh
P=/data/data/com.termux/files/usr
BT=$HOME/Android/build-tools/34.9.0
W=$P/tmp/v4probe
apkc sign --v4 --keystore "$P/tmp/studio-release.p12" --storepass saludo \
  --keyalias lanzable --out "$W/ours.apk" "$W/in.apk"
"$BT/apksigner" verify --v4-signature-file "$W/ours.apk.idsig" "$W/ours.apk"
apkc verify "$W/ours.apk"
```

```text
[sign] ours.apk carries a v1+v2+v3 signature of its own, 4 entries taken over as their bytes already were
[verify] v1 (JAR signing): ok — 4 entradas con su digest y la firma PKCS#7 verificada
[verify] v2 (APK Signature Scheme v2): ok — 7 entradas cubiertas por el digest del bloque
[verify] v4 (APK Signature Scheme v4): ok — árbol de verity y firma del .idsig
```

`apksigner verify --v4-signature-file` sale 0 con el `.idsig` que escribe este fork, tanto
sobre el apk mínimo como sobre uno de 678823 bytes (varios niveles de árbol), y también sale
0 al revés: `apkc verify` aprueba el `.idsig` que escribió `apksigner`. La página tiene una
casilla `escribir el .idsig` encendida por defecto, pasa `--v4=true` a `sign`/`build`, ofrece
la descarga del sidecar y su `apkc verify` lo comprueba; la prueba de la pestaña exporta el
`.idsig` y `studio/e2e.sh` lo coteja con `apksigner`:

```text
pestaña v4: .idsig de 5562 bytes, apksigner y apkc verify lo aprueban
```

Pruebas: `TestV4FirmaYVerifica`, `TestV4RechazaUnArbolCambiado` y
`TestV4DesactivadoNoEscribeIdsig` en `apkc/v4sign_test.go`, más el caso v4 de
`studio/prueba-pestana.js`. Frontera: una sal no vacía y los firmantes alternativos del
`.idsig` se rechazan por nombre; un `.aab` no lleva `.idsig` porque no tiene bloque v2/v3.

## 120. Rotación v3.1: el linaje y el bloque que escribe `apksigner`

Medido para 5.31, que sigue abierto: el comando que crea el linaje, el bloque v3.1 y el
atributo de prueba de rotación. No hay implementación todavía; esto es el formato que hay
que reproducir.

```sh
P=/data/data/com.termux/files/usr
BT=$HOME/Android/build-tools/34.9.0
K=$P/lib/jvm/java-21-openjdk/bin/keytool
W=$P/tmp/v31probe
"$K" -genkeypair -keystore old.p12 -storetype PKCS12 -storepass viejito -keypass viejito \
  -alias viejo -keyalg RSA -keysize 2048 -validity 3650 -dname "CN=Viejo, O=apkc, C=ES"
"$K" -genkeypair -keystore new.p12 -storetype PKCS12 -storepass nuevito -keypass nuevito \
  -alias nuevo -keyalg RSA -keysize 2048 -validity 3650 -dname "CN=Nuevo, O=apkc, C=ES"
"$BT/apksigner" rotate --out lineage.bin \
  --old-signer --ks old.p12 --ks-key-alias viejo --ks-pass pass:viejito \
  --new-signer --ks new.p12 --ks-key-alias nuevo --ks-pass pass:nuevito
"$BT/apksigner" sign --ks old.p12 --ks-pass pass:viejito \
  --next-signer --ks new.p12 --ks-pass pass:nuevito \
  --lineage lineage.bin --rotation-min-sdk-version 33 \
  --v1-signing-enabled false --v2-signing-enabled false \
  --out rotated33.apk in.apk
"$BT/apksigner" verify --verbose --min-sdk-version 28 rotated33.apk
```

```text
Verifies
Verified using v3 scheme (APK Signature Scheme v3): true
Verified using v3.1 scheme (APK Signature Scheme v3.1): true
Number of signers: 1
```

El bloque de firmas lleva tres pares:

```text
0x1b93ad61  3285   <- v3.1, el identificador medido (no 0xf05368c1)
0xf05368c0  1437   <- v3.0, para las plataformas que no leen v3.1
0x42726577  3402   <- relleno
```

El fichero `lineage.bin` (1863 bytes) es un envoltorio con magia `0x3eff39d1`, versión 1 y
la longitud del linaje interno. El linaje interno (1851 bytes) empieza con versión 1 y una
lista de nodos; cada nodo es `signedData` con longitud, `flags`, `sigAlgorithm` y `signature`
con longitud, y su `signedData` es el certificado con longitud más los `flags`. El nuevo
nodo lo firma la clave vieja. El **valor** del atributo de prueba de rotación es el linaje
interno tal cual:

| constante | valor |
| --- | --- |
| id del bloque v3.1 | `0x1b93ad61` |
| atributo `proof-of-rotation` | `0x3ba06f8c` |
| atributo `rotation-min-sdk-version` | `0x559f8b02` |
| atributo `rotation-on-dev-release` | `0xc2a6b3ba` |
| versión de rotación por defecto | 33 (Android 13) |

Con `--rotation-min-sdk-version 28` el mismo linaje va a un bloque **v3.0** (un firmante, el
nuevo, con el atributo) y `v3.1` sale `false`; con 33 salen los dos bloques y firmantes con
rangos de SDK. Las constantes del verificador ya usan `0x1b93ad61` para v3.1; escribirlo
sigue siendo 5.31.

## 121. Rotación v3.1 escrita en Go: linaje, v3.0, v3.1 y v4 de dos firmantes

El linaje de `apkc rotate` sale **byte a byte** igual al de `apksigner rotate`:

```sh
P=/data/data/com.termux/files/usr
BT=$HOME/Android/build-tools/34.9.0
W=$P/tmp/v31probe
apkc rotate --old-keystore "$W/old.p12" --old-storepass viejito --old-keyalias viejo \
  --new-keystore "$W/new.p12" --new-storepass nuevito --new-keyalias nuevo \
  --out "$W/ours-lineage.bin"
"$BT/apksigner" rotate --out "$W/lineage.bin" \
  --old-signer --ks "$W/old.p12" --ks-key-alias viejo --ks-pass pass:viejito \
  --new-signer --ks "$W/new.p12" --ks-key-alias nuevo --ks-pass pass:nuevito
cmp "$W/ours-lineage.bin" "$W/lineage.bin"
```

```text
[rotate] ours-lineage.bin links Viejo -> Nuevo
cmp rc=0
d1140cfdea6e3995cbbac67e77db05bcafbeb7de3f9eee4e5352fa9094d5da95  ours-lineage.bin
d1140cfdea6e3995cbbac67e77db05bcafbeb7de3f9eee4e5352fa9094d5da95  lineage.bin
```

Y `apksigner` acepta ese linaje para firmar una rotación (`v3` y `v3.1` true), lo que
prueba que el formato es el suyo y no solo el que lee.

Con `apkc sign --lineage` la v1/v2 las firma la clave vieja y la v3.0/v3.1 la nueva. El
bloque de firmas lleva v2, v3.1 (`0x1b93ad61`) y v3 (`0xf05368c0`), en ese orden; el
firmante v3.1 lleva el `proof-of-rotation` (`0x3ba06f8c`) y el v3.0 el
`rotation-min-sdk-version` (`0x559f8b02`) con el 33. El `.idsig` es V4.1: el firmante
viejo como principal y el nuevo en un `SigningInfoBlock` con el id `0x1b93ad61`.

```sh
apkc sign --keystore "$W/new.p12" --storepass nuevito --keyalias nuevo \
  --old-keystore "$W/old.p12" --old-storepass viejito --old-keyalias viejo \
  --lineage "$W/ours-lineage.bin" --rotation-min-sdk 33 \
  --out "$W/ours-rot33.apk" "$W/in.apk"
"$BT/apksigner" verify --verbose "$W/ours-rot33.apk"
"$BT/apksigner" verify --v4-signature-file "$W/ours-rot33.apk.idsig" "$W/ours-rot33.apk"
apkc verify "$W/ours-rot33.apk"
```

```text
Verifies
Verified using v1 scheme (JAR signing): true
Verified using v2 scheme (APK Signature Scheme v2): true
Verified using v3 scheme (APK Signature Scheme v3): true
Verified using v3.1 scheme (APK Signature Scheme v3.1): true
[apksigner --v4-signature-file] rc=0
[verify] v1 (JAR signing): ok — 4 entradas con su digest y la firma PKCS#7 verificada
[verify] v2 (APK Signature Scheme v2): ok — 7 entradas cubiertas por el digest del bloque
[verify] v4 (APK Signature Scheme v4): ok — árbol de verity y firma del .idsig
```

Con `--rotation-min-sdk 28` el mismo comando escribe un solo bloque v3 con la clave nueva
desde 28 (sin v3.1) y también verifica. La pestaña tiene la rotación en la sección de
firma: keystore viejo, contraseña, alias, «crear el linaje» y «desde qué API usa la
nueva»; el linaje y el keystore viejo quedan en OPFS y `firmar` pasa `--lineage` y
`--old-keystore`.

Pruebas: `TestElLinajeEnlazaYFirmaElNodoNuevo` y `TestLaRotacionV31EscribeV2V31V3` en
`apkc/lineage_test.go`; `studio/e2e.sh` firma una rotación con `apkc rotate` + `apkc sign`
y exige v1/v2/v3/v3.1 y el `.idsig` a `apksigner`, y `studio/prueba-pestana.js` conduce la
rotación desde la página. Frontera: `apkc verify` no verifica la firma de los bloques
v3/v3.1 (los nombra y no los aprueba); la rotación se apoya en el `.idsig` de dos firmantes
y en `apksigner` como oráculo.

## 122. EC en el firmante: los algoritmos medidos (5.32, aún abierto)

Medido con `keytool` y `apksigner` antes de escribir el firmante ECDSA. Una clave P-256 y
otra P-384, cada una en su keystore, y el mismo apk firmado con cada una:

```sh
K=$PREFIX/lib/jvm/java-21-openjdk/bin/keytool
BT=$HOME/Android/build-tools/34.9.0
"$K" -genkeypair -keystore ec.p12 -storetype PKCS12 -storepass ecpass -keypass ecpass \
  -alias ec -keyalg EC -groupname secp256r1 -validity 3650 -dname "CN=EC, O=apkc, C=ES"
"$BT/apksigner" sign --ks ec.p12 --ks-pass pass:ecpass --out ec.apk in.apk
"$K" -genkeypair -keystore ec384.p12 -storetype PKCS12 -storepass ecpass -keypass ecpass \
  -alias ec384 -keyalg EC -groupname secp384r1 -validity 3650 -dname "CN=EC384, O=apkc, C=ES"
"$BT/apksigner" sign --ks ec384.p12 --ks-pass pass:ecpass --out ec384.apk in.apk
```

| clave | bloque v2/v3 | digest del contenido | `.idsig` | firma DER |
| --- | --- | --- | --- | --- |
| P-256 | `0x0201` (ECDSA + SHA-256) | 32 bytes | `0x0201` | 71 bytes |
| P-384 | `0x0202` (ECDSA + SHA-512) | 64 bytes | `0x0202` | 102 bytes |

El PKCS#7 de la v1 de `apksigner` usa `ecdsa-with-SHA384` (OID 1.2.840.10045.4.3.3) porque
es el algoritmo con el que `keytool` firmó el certificado; el digest del `.SF` sigue siendo
SHA-256. Para este fork basta con `ecdsa-with-SHA256` (OID 1.2.840.10045.4.3.2) firmando el
SHA-256 del `.SF`, que es lo que verifica cualquier lector de JAR; el oráculo seguirá siendo
`apksigner verify` y `jarsigner -verify`.

Lo que hay que tocar: un `firmante` que envuelva `crypto.Signer` y elija
`(algID, hash, contentHash)` según la clave (RSA 0x0103/SHA-256, P-256 0x0201/SHA-256,
P-384 0x0202/SHA-512); `chunkedDigest` con SHA-256 o SHA-512; el PKCS#7 con el OID ECDSA y
la firma DER; el verificador de la v1 aceptando el OID ECDSA. El árbol de fs-verity del
`.idsig` sigue siendo SHA-256; lo que cambia es el `sigAlg` y el `apkDigest` (32 o 64).

## 123. EC en el firmante: RSA, P-256 y P-384 en el mismo camino

El firmante deja de ser una `*rsa.PrivateKey`: `apkc/firmante.go` envuelve un
`crypto.Signer` y elige el id y el hash que el formato pide. RSA va con `0x0103` y
SHA-256; P-256 con `0x0201` y SHA-256; P-384 con `0x0202` y SHA-512 (§122). El digest del
contenido se calcula con `chunkedDigest`, que acepta SHA-256 o SHA-512, y la v1 firma el
`.SF` con SHA-256 y, para EC, con el OID `ecdsa-with-SHA256`.

```sh
P=/data/data/com.termux/files/usr
BT=$HOME/Android/build-tools/34.9.0
K=$P/lib/jvm/java-21-openjdk/bin/keytool
W=$P/tmp/ecprobe
"$K" -genkeypair -keystore "$W/ec.p12" -storetype PKCS12 -storepass ecpass -keypass ecpass \
  -alias ec -keyalg EC -groupname secp256r1 -validity 3650 -dname "CN=EC, O=apkc, C=ES"
apkc sign --keystore "$W/ec.p12" --storepass ecpass --keyalias ec \
  --out "$W/ours-ec.apk" "$W/in.apk"
"$BT/apksigner" verify --verbose "$W/ours-ec.apk"
"$BT/apksigner" verify --v4-signature-file "$W/ours-ec.apk.idsig" "$W/ours-ec.apk"
apkc verify "$W/ours-ec.apk"
```

```text
Verified using v1 scheme (JAR signing): true
Verified using v2 scheme (APK Signature Scheme v2): true
Verified using v3 scheme (APK Signature Scheme v3): true
[apksigner --v4-signature-file] rc=0
[verify] v1 (JAR signing): ok — 4 entradas con su digest y la firma PKCS#7 verificada
[verify] v2 (APK Signature Scheme v2): ok — 7 entradas cubiertas por el digest del bloque
[verify] v4 (APK Signature Scheme v4): ok — árbol de verity y firma del .idsig
```

Lo mismo con `secp384r1` (0x0202, digest de 64 bytes y firma de 102): `apksigner` verifica
v1/v2/v3 y el `.idsig`, y `apkc verify` lee v1/v2/v4. La página no necesita nada
especial: el mismo selector de keystore y el mismo `firmar` sirven; el lector de PKCS#12 ya
devolvía un `crypto.Signer`, y el verificador de la v1 acepta el OID ECDSA.

Pruebas: `TestElFirmanteEligeElAlgoritmo` y `TestECFirmaYVerifica` en
`apkc/firmante_test.go`; `studio/e2e.sh` genera un keystore P-256 y otro P-384 con
`keytool`, firma con `apkc` y exige a `apksigner` v1/v2/v3 y el `.idsig` (y a `apkc
verify` la v1/v2/v4). Frontera: la rotación con claves EC se apoya en el mismo `firmante`
pero no se ha medido con `apksigner`; los dos `firmante` de una rotación pueden ser RSA.

## 124. El `minSdk` del proyecto no cambia el firmante v3 de un apk sin rotación

El plan pedía seguir el `minSdkVersion` del proyecto en el firmante v3, que este fork
escribía fijo en 24. Medido con `apksigner` para 21, 23, 24, 26, 28, 30, 33 y 34:

```sh
BT=$HOME/Android/build-tools/34.9.0
W=$PREFIX/tmp/minprobe
for api in 21 23 24 26 28 30 33 34; do
  "$BT/apksigner" sign --min-sdk-version "$api" \
    --ks "$HOME/.android/debug.keystore" --ks-pass pass:android \
    --out "$W/m$api.apk" "$W/in.apk"
done
```

En los ocho casos el firmante v3 del bloque `0xf05368c0` sale con `minSdk = 24` y
`maxSdk = 0x7fffffff`; lo que cambia con `--min-sdk-version` es la v1 (SHA-1/SHA-256) y si
la v2/v3 se escriben, no el rango del firmante. Por tanto el 24 fijo de `v3SignerBlock` ya
es el valor de `apksigner` para un apk con un solo firmante; el `minSdk` solo mueve los
rangos en una **rotación** (§120-§121), donde el v3.0 cubre hasta 32 y el v3.1 desde 33.

No hay cambio de código: la fila 5.35 se cierra con esta medición y la prueba existente que
lee el bloque v3 (`TestElBloqueV3LlevaSuParYSuSigner`) sigue exigiendo 24/0x7fffffff.

## 125. La v1 con SHA-1 para `minSdk < 18`: el formato medido (5.34, aún abierto)

Medido con `apksigner --min-sdk-version 17`; la v2/v3 siguen siendo SHA-256 y se
escriben igual, lo que cambia es la firma JAR:

```sh
BT=$HOME/Android/build-tools/34.9.0
W=$PREFIX/tmp/sha1probe
"$BT/apksigner" sign --min-sdk-version 17 \
  --ks "$HOME/.android/debug.keystore" --ks-pass pass:android \
  --out "$W/sha1.apk" "$W/in.apk"
unzip -p "$W/sha1.apk" META-INF/MANIFEST.MF
unzip -p "$W/sha1.apk" META-INF/ANDROIDD.SF
```

```text
Manifest-Version: 1.0
Name: AndroidManifest.xml
SHA1-Digest: jKpiXWWzk3Ck8KoIm7/CjVjc/qQ=
...
Signature-Version: 1.0
Created-By: 1.0 (Android)
SHA1-Digest-Manifest: yQzlWEi2quoIg9IeuGKD47AbJT8=
X-Android-APK-Signed: 2, 3
...
SHA1-Digest: IP1zEMhTBWw6she5ykj6M5hIMJk=
```

Dos detalles que no se adivinan: el atributo es **`SHA1-Digest`**, no `SHA-1-Digest`, y
el `.SF` lleva `X-Android-APK-Signed: 2, 3` cuando el paquete también tiene v2/v3. El
PKCS#7 usa `sha1` (OID 1.3.14.3.2.26) como digest y `rsaEncryption` como algoritmo de
firma; para EC sería `ecdsa-with-SHA1` (1.2.840.10045.4.1) y la firma DER. `apksigner
verify` sigue diciendo v1/v2/v3 `true`.

Lo que falta para escribirlo: un modo `v1SHA1` que cambie los nombres de atributo, el
digest del manifiesto y del `.SF`, el OID y el hash de la firma; los mapas de digests SHA-1
para las entradas, que en el build tienen que convivir con la reutilización de un paquete
anterior (hoy el mapa es SHA-256), y la lectura del verificador para el OID SHA-1.

## 126. La v1 con SHA-1, escrita y verificada

El modo `v1SHA1` cambia solo la firma JAR: el manifiesto usa `SHA1-Digest` (sin guion), el
`.SF` `SHA1-Digest-Manifest`/`SHA1-Digest` y `X-Android-APK-Signed: 2, 3`, y el PKCS#7
firma el SHA-1 del `.SF` con `sha1`/`rsaEncryption` (o `ecdsa-with-SHA1`). La v2/v3 siguen
con SHA-256 y el `.idsig` de la v4 no cambia.

```sh
P=/data/data/com.termux/files/usr
BT=$HOME/Android/build-tools/34.9.0
apkc sign --v1-sha1 --keystore "$HOME/.android/debug.keystore" --storepass android \
  --out "$P/tmp/sha1probe/ours-sha1.apk" "$P/tmp/sha1probe/in.apk"
"$BT/apksigner" verify --verbose --min-sdk-version 17 "$P/tmp/sha1probe/ours-sha1.apk"
apkc verify "$P/tmp/sha1probe/ours-sha1.apk"
```

```text
Manifest-Version: 1.0
Name: AndroidManifest.xml
SHA1-Digest: jKpiXWWzk3Ck8KoIm7/CjVjc/qQ=
...
Signature-Version: 1.0
Created-By: 1.0 (Android)
SHA1-Digest-Manifest: yQzlWEi2quoIg9IeuGKD47AbJT8=
X-Android-APK-Signed: 2, 3
...
Verified using v1 scheme (JAR signing): true
Verified using v2 scheme (APK Signature Scheme v2): true
Verified using v3 scheme (APK Signature Scheme v3): true
[verify] v1 (JAR signing): ok — 4 entradas con su digest y la firma PKCS#7 verificada
[verify] v2 (APK Signature Scheme v2): ok — 7 entradas cubiertas por el digest del bloque
[verify] v4 (APK Signature Scheme v4): ok — árbol de verity y firma del .idsig
```

Los digests SHA-1 de cada entrada y del manifiesto son **los mismos** que escribe
`apksigner` para el mismo apk (`jKpi…`, `IP1z…`, `yQzl…`), y `apkc verify` aprueba también
el sha1.apk que firmó `apksigner` con `--min-sdk-version 17`. En el build, un proyecto con
`minSdkVersion < 18` enciende `v1SHA1` solo; `--v1-sha1` lo fuerza. Para poder calcular el
mapa SHA-1, un build en ese modo desactiva la reutilización de entradas (vuelve a escribir
todas) y un `sign` relee las del paquete de origen; el mapa SHA-256 sigue siendo el de
v2/v3/v4.

Pruebas: `TestV1SHA1FirmaYVerifica` en `apkc/v1sha1_test.go`; `studio/e2e.sh` firma con
`--v1-sha1`, exige `SHA1-Digest`/`SHA1-Digest-Manifest`, y coteja v1/v2/v3 con
`apksigner --min-sdk-version 17` y la v1 con `apkc verify`.

## 127. El `.aab`: qué es lo que `bundletool` añade al `bundle.zip` (5.33, en marcha)

El `bundle.zip` que deja el build con aapt2 ya viene en formato **protobuf**: `manifest/
AndroidManifest.xml`, `dex/classes.dex`, `resources.pb` y `res/...` (el layout también es
proto XML). `bundletool build-bundle` solo lo envuelve con `BundleConfig.pb` y el prefijo
`base/`, y `jarsigner` le pone la firma JAR. Medido:

```sh
apkc build --aab --keystore "$HOME/.android/debug.keystore"   # camino con aapt2
unzip -l build/bundle.zip
unzip -l build/app.aab
unzip -p build/app.aab BundleConfig.pb | xxd
java -jar "$HOME/Android/bundletool/bundletool-all-1.18.3.jar" validate --bundle build/app.aab
```

```text
bundle.zip: manifest/AndroidManifest.xml 1737 · dex/classes.dex 2844 ·
            resources.pb 1020 · res/layout/main.xml 1251
app.aab:    BundleConfig.pb 10 · base/{manifest,dex,res,resources.pb} + META-INF
BundleConfig.pb: 0a08 1206 312e 3138 2e33        # bundletool 1.18.3
validate rc=0
```

El `.aab` no lleva bloque de firmas v2/v3: solo la JAR v1, que ya sabe escribir este fork
(§118). Por eso 5.33 se puede cerrar en dos piezas: el **empaquetado** (leer el
`bundle.zip` proto, añadir `BundleConfig.pb` y prefijar `base/`) y la **firma** con
`resignBundleCon`, sin `bundletool` ni `jarsigner`. Lo que sigue fuera es la generación
del propio `bundle.zip` protobuf desde el enlazador Go: aapt2 es quien produce
`resources.pb` y el proto XML; el linker de este fork escribe `resources.arsc`/AXML, y
convertirlo es una fase aparte. El oráculo del empaquetado es `bundletool validate` y
`jarsigner -verify`.

## 128. El `.aab` ensamblado y firmado en Go, sin `bundletool` ni `jarsigner`

`apkc build --aab` deja de arrancar `bundletool` y `jarsigner`: `apkc/aab.go` lee el
`bundle.zip` que escribe el enlazador aapt2 (ya en protobuf), añade `BundleConfig.pb` y
prefija cada entrada con `base/`; después firma el ZIP como JAR v1 con `resignBundleCon`,
la misma pieza de la 5.29. El `BundleConfig.pb` es el mensaje medido en el §127
(`0a08 1206 "1.18.3"`).

```sh
apkc build --aab --keystore "$HOME/.android/debug.keystore"
unzip -l build/app.aab
java -jar "$HOME/Android/bundletool/bundletool-all-1.18.3.jar" validate --bundle build/app.aab
jarsigner -verify build/app.aab
apkc verify build/app.aab
```

```text
BundleConfig.pb 10 · base/manifest/AndroidManifest.xml · base/dex/classes.dex ·
base/resources.pb · base/res/layout/main.xml · META-INF/{MANIFEST.MF,ANDROID.SF,ANDROID.RSA}
App Bundle information · Feature module: base · validate rc=0
jar verified. (aviso de cadena autofirmada, rc=0)
[verify] v1 (JAR signing): ok — 5 entradas con su digest y la firma PKCS#7 verificada
[verify] el paquete es un .aab: la firma es JAR v1 y no lleva bloque v2/v3
```

El `.aab` no lleva bloque de firmas v2/v3 ni se zipaligna: es un bundle, y su única firma
es la JAR v1. `TestBundleConfigPB` y `TestElAABEnsamblaElBundleProto` en `apkc/aab_test.go`
cubren el mensaje y el ensamblado más la firma (con el verificador propio reconociendo el
bundle); `studio/e2e.sh` ejecuta `build --aab` en la variante `java` y exige
`bundletool validate`, `jarsigner -verify` con `jar verified` y `apkc verify`.

Frontera: el `bundle.zip` protobuf lo sigue produciendo aapt2. El enlazador Go de este
fork escribe AXML y `resources.arsc`, no proto XML ni `resources.pb`; convertirlo es una
fase aparte, así que el `.aab` de la pestaña (que compila con `--linker go`) todavía no se
puede armar sin aapt2. La pestaña firma `.aab` que ya vienen en protobuf (§118).

## 129. La caída de un brazo del `switch` al siguiente

Medido sobre `javac --release 8` y comprobado con el dex. Lo que cambia no es la tabla: los
cuerpos se emiten **en el orden del fuente** (no en el de la clave), el `default` se queda donde
el fuente lo pone, y solo el brazo que acaba en `break` lleva su `goto` al final —y el último
brazo no lo lleva, porque cae ya en el final del `switch`—.

```sh
P=/data/data/com.termux/files/usr
JDK=$P/lib/jvm/java-21-openjdk/bin
W=$P/tmp/jprobe5
"$JDK/javac" --release 8 -g:lines,source -d "$W/cae/classes" "$W/cae/src/p/Cae.java"
"$JDK/javap" -c -p "$W/cae/classes/p/Cae.class"
```

```text
case 0: total = 1;              32: iconst_1
case 1: total = 2; break;       33: putstatic
                                36: iconst_2
                                40: goto 47
case 5: total = 5;              28: iconst_5    (lookupswitch 5: 28, 20: 32)
case 20: total = 2; break;      29: putstatic
                                32: iconst_2
                                36: goto 44
default: total = 9;             28: bipush 9   (default primero, cae en case 1)
case 1: total = 1; break;       33: iconst_1
```

`TestLaCaidaAlSiguienteBrazo` (`apkc/javaemisor_switch_test.go`) compila ese fuente con el
emisor y exige que el dex sea **byte a byte** el de las clases de javac. El rechazo que había
por nombre desaparece de `TestElSwitchDelFuenteRechazaLoNoMedido`.

## 130. `ConstantValue` ancho y la cuenta de `float`/`double` en tres registros

Dos piezas de la fila 5.8. Primero el atributo: `javac` dobla en `ConstantValue` también los
`static final` que no son `int`/`char`, sin bajarlos al `<clinit>`.

```sh
"$JDK/javac" --release 8 -g:lines,source -d "$W/cf/classes" "$W/cf/src/p/Constantes.java"
"$JDK/javap" -v -p "$W/cf/classes/p/Constantes.class"
```

```text
static final long L;    ConstantValue: long 6l       (tag 5)
static final float F;   ConstantValue: float 2.5f    (tag 4)
static final double D;  ConstantValue: double 3.5d   (tag 6)
static final String S;  ConstantValue: String hola   (tag 8)
```

Segundo, la forma 23x: `d8` la escribe cuando un operando no cabe en un nibble, por ejemplo
nueve `double` de parámetro (`i` en v16): `ab00 0010` = `add-double v0, v0, v16`. El traductor
la emite ahora con destino en el operando muerto y, si los dos están vivos, en un par temporal.
La sonda `a - (a - c)` fuerza la forma porque `a` está vivo en la pila al hacer la resta
interior: `ac00 0402` = `sub-double v0, v4, v2`.

`TestElConstantValueLargoFlotanteDobleYCadena` y `TestLaCuentaAnchaDeTresRegistros`
(`apkc/javaemisor_tres_registros_test.go`) exigen el dex byte a byte igual al de javac y, en la
segunda, que el cuerpo lleve `sub-double` 0xac y `sub-float` 0xa7 en la forma 23x.

## 131. `instanceof T[]` y la forma negada

```sh
"$JDK/javac" --release 8 -g:lines,source -d "$W/ti/classes" "$W/ti/src/p/Tipo.java"
"$JDK/javap" -c -p "$W/ti/classes/p/Tipo.class"
```

```text
o instanceof String[]   -> 1: instanceof #7  // class "[Ljava/lang/String;"
                           4: ifeq 9
!(o instanceof String)  -> 1: instanceof #9  // class java/lang/String
                           4: ifne 9
!(o instanceof int[])   -> 1: instanceof #11 // class "[I"
                           4: ifne 10
```

El tipo probado es el descriptor del arreglo (`[Ljava/lang/String;`, `[I`) y la negación no
añade `iconst`/`ixor`: el mismo `instanceof` y la branquia invertida (`ifne`). `TestInstanceOfDeUnArregloYSuFormaNegada`
(`apkc/javaemisor_instanceof_arreglo_test.go`) exige el dex byte a byte igual al de javac y que
nombren los dos descriptores. `TestElInstanceofRechazaLoNoMedido` conserva el rechazo del
`instanceof` sobre un entero.

## 132. `catch (A | B e)` y el `continue` que sale de un `try` con `finally`

El multi-catch de `javac` es **una fila de la tabla por tipo y un solo manejador**: la excepción
se guarda una vez (`astore_1`) y el cuerpo está una vez.

```text
Exception table:
   from    to  target type
       0    16    27   Class java/lang/IllegalArgumentException
       0    16    27   Class java/lang/IllegalStateException
       0    16    44   any
      27    33    44   any
```

El emisor escribe cada tipo en `tipos` de la captura, un solo bloque de handler y una fila por
descriptor; el hueco de la excepción es `Throwable` (cualquier excepción es asignable y el dex no
guarda el tipo del local porque no hay tabla de variables). Con `finally` el `any` del cuerpo y el
del catch salen en el mismo orden que javac.

El `continue` que sale de un `try` con `finally` es como el `break`: la copia del bloque delante
del `goto`, la línea del `continue` en el salto y el tramo protegido cortado después del salto.

```sh
"$JDK/javac" --release 8 -g:lines,source -d "$W/mu/classes" "$W/mu/src/p/Multi.java"
"$JDK/javac" --release 8 -g:lines,source -d "$W/co/classes" "$W/co/src/p/Cont.java"
```

`TestElCatchDeVariosTipos` y `TestElContinueQueSaleDeUnTryConFinally` comparan el dex byte a byte
con el de javac. Frontera que sigue abierta: los **dos `finally` anidados con salida pendiente**
(§133), donde javac solapa rangos de la tabla y el dex no los admite solapados.

## 133. Los dos `finally` anidados con salida pendiente: sigue fuera, y por qué

El emisor rechaza el caso con su nombre:

```sh
apkc compile --android-jar "$ANDROID_JAR" -o "$W/nd/out" "$W/nd/src/p/D.java"
```

```text
[compile] D.java: linea 2: linea 4: linea 5: un `return` que cruza otro `try` anidado no esta medido
```

`javac` lo escribe con la tabla **solapada**, en orden de dentro hacia fuera (la primera fila
que casa es la que gana):

```text
Exception table:
   from    to  target type
       0     6    42   any     <- finally interior
      26    30    42   any
       0    15    66   any     <- finally exterior
      26    54    66   any
```

El dex no admite `try_item` solapados, así que escribirlo pide dos cosas que no están hechas:
normalizar la tabla en `armaTries` (cortar por los extremos y quedarse, en cada tramo, con el
primer `any` que casa) y que `intenta` copie los dos bloques seguidos en cada salida, como hace
`javac`. Medido y dejado fuera a propósito; la fila 5.9 lo dice.

## 134. Las piezas nuevas ejecutadas en ART

Además del dex byte a byte, dos sondas con `main` se compilan con el emisor, se dexan con el
fork y se corren en el aparato con `app_process64`, comparando con la JVM:

```sh
apkc compile --android-jar "$ANDROID_JAR" -o "$W/cie/our" "$W/cie/src/p/Cierre.java"
apkc dex -o "$W/cie/dex/classes.dex" "$W/cie/our"
java -cp "$W/cie/jc" p.Cierre
sudo sh -c '. /data/local/tmp/preludio.sh; export CLASSPATH=/data/local/tmp/cierre.dex;
            /system/bin/app_process64 /data/local/tmp p.Cierre'
```

```text
JVM: 2121045997  (rc 0)
ART: 2121045997  (rc 0)
dexdump: rc 0
```

`Cierre` junta la caída del `switch` (`cae`), el `continue` que sale de un `try` con `finally`
(`cont`), el `catch (A | B e)` (`multi`) y la resta ancha en tres registros (`ancha`). Y la
sonda `Tipo` corre en ART con `instance-of` sobre `[Ljava/lang/String;` y `[I`:

```text
instance-of v0, v2, [Ljava/lang/String; // type@0008
instance-of v0, v2, Ljava/lang/String;  // type@0003
instance-of v0, v2, [I                 // type@0007
JVM: 108 (rc 0)   ART: 108 (rc 0)
```

## 135. `format="enum"` y `format="flags"` con hijos

El mapa de un atributo con hijos no es solo la marca de formato: cada hijo es un recurso `id` y el
registro lo referencia con ese id; el valor es el entero.

```sh
W=$PREFIX/tmp/enumprobe
BT=$HOME/Android/build-tools/34.9.0
"$BT/aapt2" compile --dir "$W/res" -o "$W/compiled.zip"
"$BT/aapt2" link -o "$W/out/app.apk" -I "$HOME/Android/platforms/android-34/android.jar" \
  --manifest "$W/AndroidManifest.xml" --no-auto-version "$W/compiled.zip"
"$BT/aapt2" dump resources "$W/out/app.apk"
```

```text
type attr id=01 entryCount=2
  resource 0x7f010000 attr/banderas
    () (attr) type=flags size=2
      alta(0x7f020000)=0x00000001
      baja(0x7f020001)=0x00000002
  resource 0x7f010001 attr/modo
    () (attr) type=enum size=2
      dos(0x7f020002)=0x00000002
      uno(0x7f020003)=0x00000001
type id id=02 entryCount=4
  resource 0x7f020000 id/alta … id/uno
```

Las marcas son `0x00010000` (enum) y `0x00020000` (flags), la misma familia que los bits de
`format` ya medidos. Los registros van ordenados por el nombre del hijo (que es el orden de los
ids) y los recursos `id` llevan el valor booleano a cero, como un `<item type="id">`. Los ids se
resuelven con una pasada previa sobre los tipos y claves ordenados, porque el mapa del atributo
necesita el número del `id` antes de escribir la tabla. `TestAttrEnumYFlagsConHijos`
(`apkc/valores_enum_test.go`) exige el arsc **byte a byte** el de aapt2 (896 bytes).

## 136. Instalar la app de cierre destapó dos fallos del emisor

La app de cierre se armó con el camino todo-Go y se instaló en la pantalla secundaria. El
primer intento murió en `onCreate`, y el logcat dio el motivo exacto.

```sh
W=$PREFIX/tmp/cierre-app
apkc build --linker go --compiler go --signer go \
  --android-jar "$ANDROID_JAR" --keystore "$HOME/.android/debug.keystore"
virtual_screen app install --open "$W/build/app.apk"
virtual_screen app logs com.icono
```

```text
java.lang.IncompatibleClassChangeError: The method 'int com.icono.MainActivity.calcula()'
was expected to be of type virtual but instead was found to be of type static
```

El emisor trataba `calcula()` (un `static` llamado sin receptor desde un método de instancia)
como `this.calcula()` y escribía `invokevirtual`. Corregido: cuando el candidato es estático se
escribe `invokestatic` sin cargar `this`. `TestLosEstaticosSeLlamanSinReceptor` compara el dex
con el de javac.

El segundo intento dio otro error, también del emisor:

```text
java.lang.NoSuchMethodError: No direct method <init>(Lcom/icono/MainActivity;)V in class
Landroid/widget/TextView;
```

`new TextView(this)` se firmaba con el descriptor del **argumento**
(`(Lcom/icono/MainActivity;)V`) en vez del de la **declaración**
(`(Landroid/content/Context;)V`). Corregido: el constructor se resuelve con
`buscarMetodo` y se usa `cand.desc`. `TestElConstructorSeFirmaConSuDescriptor` lo fija con una
jerarquía `A`/`B` y un `new A(b)`.

Con las dos correcciones la app arranca, con su etiqueta en español y el texto que calcula:

```text
com.icono/com.icono.MainActivity visible=True tarea=594
barra de título: «Cierre ES»   TextView: «total 203»
```

La cuenta del `onCreate`: el bucle deja `n = 10`, el `switch` cae en `case 10` (`total = 110`),
`+= 'A'` suma 65, el `catch` suma 7, y `seis(1..6)` suma 21 → 203. Es el cierre de la Fase 5:
bucle, `switch`, `try`/`catch`, `char`, `+=`/`++`, un método de seis argumentos, icono y dos
idiomas, compilado, empaquetado y firmado por el fork, instalado y ejecutado.

## 137. Más densidades y la orientación `port`

Medido con aapt2 sobre un proyecto con base, `drawable-ldpi/mdpi/xhdpi/xxhdpi`, `layout-land` y
`layout-port`:

```sh
W=$PREFIX/tmp/dens3
BT=$HOME/Android/build-tools/34.9.0
"$BT/aapt2" compile --dir "$W/res" -o "$W/c.zip"
"$BT/aapt2" link -o "$W/out/app.apk" -I "$HOME/Android/platforms/android-34/android.jar" \
  --manifest "$W/AndroidManifest.xml" --no-auto-version "$W/c.zip"
```

| configuración | `ResTable_config` | spec |
| --- | --- | --- |
| ldpi | density 120 en +14, sdkVersion 4 en +24 | `0x0500` |
| mdpi | density 160, sdkVersion 4 | `0x0500` |
| xhdpi | density 320, sdkVersion 4 | `0x0500` |
| xxhdpi | density 480, sdkVersion 4 | `0x0500` |
| port | orientation 1 en +12 | `0x0080` |
| land | orientation 2 en +12 | `0x0080` |

Dos detalles que no se ven en el dump de aapt2: el spec del drawable sale `0x0500` **cuando la
clave tiene base** (con la clave solo en la variante, aapt2 la descarta) y las configuraciones se
ordenan por su valor, no por su nombre: `port` (1) antes que `land` (2), y las densidades de menor
a mayor (120, 160, 320, 480), no por los bytes little-endian de +14. `configDe` escribe la
densidad y `sdkVersion`, `mascaraConfig` da la bandera y `claveDeConfig` fija el orden.
`apkc/densidades` acepta `layout-port` y `drawable-{ldpi,mdpi,hdpi,xhdpi,xxhdpi,xxxhdpi}`; la ruta
del apk lleva `-v4` como aapt2. `TestDensidadesYOrientacionPort`
(`apkc/valores_densidades_test.go`) exige el arsc **byte a byte** el de aapt2 (1568 bytes).

Frontera: `nodpi`/`tvdpi`, los calificadores de tamaño (`sw600dp`, `w600dp`) y la copia
`layout-v1` que el aapt2 por defecto añade siguen fuera.

## 138. Los atributos de `<permission>`: `flags` se escribe en hexadecimal

Un `<permission>` con `label`, `description`, `icon` y `protectionLevel` no rompía el enlace,
pero el AXML no era el de aapt2: `protectionLevel="dangerous"` salía como entero decimal y
aapt2 lo escribe hexadecimal.

```sh
W=$PREFIX/tmp/permprobe
BT=$HOME/Android/build-tools/34.9.0
apkc build --linker go --compiler go --signer go \
  --android-jar "$ANDROID_JAR" --keystore "$HOME/.android/debug.keystore"
"$BT/aapt2" dump xmltree "$W/build/app.apk" --file AndroidManifest.xml
"$BT/aapt2" dump xmltree "$W/out/app.apk" --file AndroidManifest.xml
```

```text
nuestro:  …:protectionLevel(0x01010009)=1
aapt2:    …:protectionLevel(0x01010009)=0x00000001
```

El tipo no se copia del item del framework (ahí `dangerous` está guardado como 0x10): lo decide
el **formato** del atributo. Los de `format="flags"` (`gravity`, `protectionLevel`) van con
`0x11`; los `enum` (`orientation`, `layout_width`) con `0x10`. `marcoRecursos` guarda ahora la
máscara de formato de cada atributo (el registro `0x01000000` que antes se descartaba) y
`valorDeAtributo` cambia a hexadecimal cuando el bit `0x00020000` está puesto.

Con eso el `AndroidManifest.xml` sale **byte a byte** el de aapt2 (1364 bytes) y el
`resources.arsc` también. `TestPermisoConSusAtributos` (`apkc/enlazar_permiso_test.go`) lo fija
con el `android.jar` real. Frontera: los valores compuestos (`protectionLevel="signature|privileged"`)
siguen fuera; solo se resuelve un nombre por atributo.

Al intentar cerrarlo se midió el segundo motivo, que no es la tabla sino el **orden del código**:
este emisor escribe el manejador del `try` interior en línea, dentro del cuerpo del exterior, así
que el rango `any` exterior taparía el `athrow` de ese manejador; `javac` coloca los manejadores
después del camino normal y sus rangos los excluyen a propósito. Igualar el dex pide mover la
emisión de los manejadores (y las copias) al final del constructo, no solo normalizar la tabla.
Por eso la fila 5.9 sigue con esa frontera y el rechazo por nombre se conserva.

## 139. El `finally` anidado con salida pendiente: copias y manejadores como javac

La forma que faltaba. `javac` copia los bloques de **dentro a fuera** en cada salida, deja la copia
del interior dentro del rango del exterior, y solapa las filas de la tabla:

```text
f(0):  return 1  ->  istore_1; finally interior; finally exterior; iload_1; ireturn
Exception table de f:
   0    6   42  any      <- interior
  26   30   42  any
   0   15   66  any      <- exterior (incluye la copia del interior)
  26   54   66  any
Exception table de g (continue/break):
   9   14   57  any      9   19   73  any
  27   32   57  any     27   37   73  any
  45   49   57  any     45   65   73  any
```

El emisor copia la cadena (`copiaLosFinally`, de dentro a fuera) y cierra el tramo de cada `try`
antes de su copia, así que el rango exterior incluye la copia interior. El `athrow` que queda al
final del cuerpo es el del manejador del `try` interior, y por eso ya no dispara el rechazo del
`throw` del fuente: `terminaEnTry` distingue el caso. Un `try` interior **con `catch`** que cruce
la salida sigue rechazado por nombre.

El dex no admite `try_item` solapados: `armaTriesSolapados` corta por todos los extremos y en cada
tramo se queda con las filas que lo cubren hasta la primera `any` (la de dentro), y funde tramos
contiguos con la misma lista.

```sh
apkc compile --android-jar "$ANDROID_JAR" -o "$W/nf2/our" "$W/nf2/src/p/Cierre2.java"
apkc dex -o "$W/nf2/dex/classes.dex" "$W/nf2/our"
java -cp "$W/nf2/jc" p.Cierre2
sudo sh -c '. /data/local/tmp/preludio.sh; export CLASSPATH=/data/local/tmp/cierre2.dex;
            /system/bin/app_process64 /data/local/tmp p.Cierre2'
sudo /apex/com.android.art/bin/dexdump /data/local/tmp/cierre2.dex
```

```text
JVM: 1 110 0 112 330   (rc 0)
ART: 1 110 0 112 330   (rc 0)
dexdump: rc 0   (el método f declara 4 filas de catch)
```

`TestLaSalidaQueCruzaDosFinally` (`apkc/javaemisor_finally_anidado_test.go`) compila un `return`
y un `continue`/`break` que cruzan los dos bloques y exige el dex **byte a byte** el de javac.

