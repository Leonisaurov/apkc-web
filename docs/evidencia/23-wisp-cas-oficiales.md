## 349. CAs oficiales del propio origen para el módulo WISP, con huella fija

Medida el 2026-11-24 en el worktree `wisp-cas-oficiales-impl`, sobre el submódulo `studio/`
(commit `bd99a0b06880d66313ab40a929f116dd874fdfa5`), sin chromium, sin CDP, sin gate, sin red,
sin `git push` y sin publicar. Cierra el plan aprobado
`…/plan-wisp-cas-oficiales/PLAN-wisp-cas-oficiales.md` de la sesión `ses_def6aa358ffe52VV0dDnWRezPH`.
Re-medida el 2026-11-24 en el worktree `cas-desde-curl` (submódulo `studio/` en
`8788604081945ed58314d333fa5b06e97eb2b7b3`), con descarga real del `cacert.pem` de curl: cambia
la procedencia y el texto PEM (el DER de los tres roots es el mismo), y por eso cambia la huella
fija. Se conserva el número `§349`. Sin chromium, sin CDP, sin gate, sin `git push` y sin publicar.
Revierte la forma «solo `File` local» de `§257`/`§258`: las CAs oficiales de los dos hosts
autorizados (`apkc/depswisp.go:90-93`) entran como bundle incrustado en el propio origen; el `File`
local sigue como override. El módulo **sigue opt-in y apagado por defecto**; el relay jamás es
fuente de confianza y no hay `fetch` de CAs.

### La medición TLS (reabierta en la re-medida con el bundle de curl)

La sesión `ses_def6dfe01ffe7a5G21zMMH5eEp` midió las cadenas con estos comandos:

```sh
go version
openssl s_client -showcerts -connect repo1.maven.org:443 -servername repo1.maven.org </dev/null 2>&1 | grep -E '^(verify | [0-9] [si]:)'
openssl s_client -showcerts -connect repo.maven.apache.org:443 -servername repo.maven.apache.org </dev/null 2>&1 | grep -E '^(verify | [0-9] [si]:)'
```

- `repo1.maven.org`: cadena `repo1 ← WE1 ← GTS Root R4 ← GlobalSign Root CA`. El servidor presenta
  GTS R4 **cross-signed**, así que se ancla con la versión **self-signed** del store; con
  GlobalSign R3/R6 `x509.Verify` falla y con GTS R4 self-signed da **rc 0**.
- `repo.maven.apache.org`: cadena `repo.maven.apache.org ← YE1 ← Root YE ← ISRG Root X2 ← ISRG
  Root X1`. Se ancla con ISRG Root X1 y, opcionalmente, X2 self-signed; `x509.Verify` da **rc 0**
  sobre el toolchain fijado. En la re-medida se relanzó con el bundle extraído de curl y el Go
  fijado, sin CDP ni chromium; el programa solo ancla esos tres PEM en un `x509.CertPool` y hace
  `tls.Dial` con `RootCAs` a cada host, de modo que el handshake ejecuta `x509.Verify`:

```sh
tcr -c 2 -m 2G env GOTMPDIR=$HOME/.cache/apkc-web/gotmp /home/leonisauro/sdk/go1.24.0/bin/go run $HOME/.cache/apkc-web/cas-curl/verificar.go
```

Salida literal:

```
repo1.maven.org: x509.Verify rc 0, VerifiedChains=1, handshake=true, raices=3
repo.maven.apache.org: x509.Verify rc 0, VerifiedChains=2, handshake=true, raices=3
```

### Toolchain fijado

```sh
/home/leonisauro/sdk/go1.24.0/bin/go version
```

Salida literal:

```
go version go1.24.0 linux/amd64
```

El `go` del sistema da `go version go1.27.4-X:nodwarf5 linux/amd64` (`which go` → `/usr/bin/go`):
no es el fijado en §312 y no se usa para este repo. El `x509.Verify` rc 0 de arriba se obtuvo con
Go 1.24.0, no con ese Go del sistema.

### Procedencia: el `cacert.pem` oficial de curl

```sh
mkdir -p "$HOME/.cache/apkc-web/cas-curl"
curl -fSL --retry 2 -o "$HOME/.cache/apkc-web/cas-curl/cacert.pem" https://curl.se/ca/cacert.pem
sha256sum "$HOME/.cache/apkc-web/cas-curl/cacert.pem"
wc -c "$HOME/.cache/apkc-web/cas-curl/cacert.pem"
grep -c 'BEGIN CERTIFICATE' "$HOME/.cache/apkc-web/cas-curl/cacert.pem"
```

`curl -sS -I https://curl.se/ca/cacert.pem` responde `HTTP/2 200`,
`content-type: application/x-pem-file` y `last-modified: Thu, 05 Nov 2026 04:12:01 GMT`; la
descarga terminó con `100 195.4k 100 195.4k ... 1.57M`. Salida literal de las tres comprobaciones:

```
427c064216761a1e72a8b30f0d954d08cb6e4435a82091fb865afd20c8497285  /home/leonisauro/.cache/apkc-web/cas-curl/cacert.pem
200116 /home/leonisauro/.cache/apkc-web/cas-curl/cacert.pem
118
```

La fecha del bundle va en su cabecera: `## Certificate data from Mozilla as of: Thu Nov  5
04:12:01 2026 GMT` (`head -8` del fichero).

Extracción por subject. curl intercala texto descriptivo y líneas `#` entre bloques, así que el
`awk` recorta desde `-----BEGIN CERTIFICATE-----` hasta `-----END CERTIFICATE-----`:

```sh
cd "$HOME/.cache/apkc-web/cas-curl"
awk '/-----BEGIN CERTIFICATE-----/{dentro=1; n++} dentro {print > sprintf("cert-%03d.pem", n)} /-----END CERTIFICATE-----/{dentro=0}' cacert.pem
for f in cert-*.pem; do echo "== $(basename "$f")"; openssl x509 -noout -subject -issuer -fingerprint -sha256 -in "$f"; done
```

Los tres bloques elegidos, con subject, issuer y fingerprint SHA-256 del certificado (salida
literal, recortada a los tres que interesan):

```
== cert-083.pem
subject=C=US, O=Google Trust Services LLC, CN=GTS Root R4
issuer=C=US, O=Google Trust Services LLC, CN=GTS Root R4
sha256 Fingerprint=34:9D:FA:40:58:C5:E2:63:12:3B:39:8A:E7:95:57:3C:4E:13:13:C8:3F:E6:8F:93:55:6C:D5:E8:03:1B:3C:7D
== cert-042.pem
subject=C=US, O=Internet Security Research Group, CN=ISRG Root X1
issuer=C=US, O=Internet Security Research Group, CN=ISRG Root X1
sha256 Fingerprint=96:BC:EC:06:26:49:76:F3:74:60:77:9A:CF:28:C5:A7:CF:E8:A3:C0:AA:E1:1A:8F:FC:EE:05:C0:BD:DF:08:C6
== cert-078.pem
subject=C=US, O=Internet Security Research Group, CN=ISRG Root X2
issuer=C=US, O=Internet Security Research Group, CN=ISRG Root X2
sha256 Fingerprint=69:72:9B:8E:15:A8:6E:FC:17:7A:57:AF:B7:17:1D:FC:64:AD:D2:8C:2F:CA:8C:F1:50:7E:34:45:3C:CB:14:70
```

Los tres son **self-signed** (`subject` = `issuer`): no se eligió ninguna versión cross-signed. El
servidor de `repo1.maven.org` presenta GTS R4 cross-signed, que se ancla con esta versión
self-signed; el de `repo.maven.apache.org` se ancla con X1 y, opcionalmente, X2 self-signed, con
`x509.Verify` **rc 0** medido arriba sobre Go 1.24.0. Los fingerprints son idénticos a los del
store local, así que el DER es el mismo y esa validación sigue siendo la suya:

```sh
openssl x509 -noout -subject -issuer -fingerprint -sha256 -in /etc/ca-certificates/extracted/cadir/GTS_Root_R4.pem
openssl x509 -noout -subject -issuer -fingerprint -sha256 -in /etc/ca-certificates/extracted/cadir/ISRG_Root_X1.pem
openssl x509 -noout -subject -issuer -fingerprint -sha256 -in /etc/ca-certificates/extracted/cadir/ISRG_Root_X2.pem
```

dan los mismos tres fingerprints `34:9D:FA…:7D`, `96:BC:EC…:C6` y `69:72:9B…:70`. Las huellas
conocidas `7e8b80d0…91776a2`, `22b557a2…543d1` y `a13d881e…b013` son los `sha256sum` de los
**ficheros PEM** locales (con sus saltos de línea), no los fingerprints del DER: con otro ancho de
línea cambian las primeras aunque el certificado sea idéntico. Los `sha256sum` de los bloques de
curl son `cc506b1e8f94414acd6afecc6784758cd3204be35426754646b6f1863dfdb158` (R4),
`253cd971fe376c24a225953cb9a336cbe36e07f7c5becbc4b954be3a256c46df` (X1) y
`f51348d1586536068882d64922b8a6d69402b14e46aecd766da1d52274619767` (X2).

Concatenación en orden R4, X1, X2, sin líneas `#`:

```sh
cat cert-083.pem cert-042.pem cert-078.pem > bundle-curl.pem
wc -c bundle-curl.pem
sha256sum bundle-curl.pem
grep -c 'BEGIN CERTIFICATE' bundle-curl.pem
grep -c '^#' bundle-curl.pem
```

Salida literal:

```
3487 /home/leonisauro/.cache/apkc-web/cas-curl/bundle-curl.pem
c2b67c490da9b82d68b77f88c924288faa78fcc66077088c530921053ff19a2c  /home/leonisauro/.cache/apkc-web/cas-curl/bundle-curl.pem
3
0
```

(`grep -c '^#'` devuelve `0` con código de salida 1: la orden compuesta termina con código 1.)
`diff -u` entre este bundle y el antiguo de p11-kit muestra solo el ancho de línea: 64 caracteres
en p11-kit, 80 en curl. El texto base64 y el DER son los mismos; por eso el tamaño baja de 3494 a
3487 bytes y cambia la huella del fichero.

### Contraste con la medición original (p11-kit, ya no es la procedencia)

```sh
sha256sum /etc/ca-certificates/extracted/cadir/GTS_Root_R4.pem /etc/ca-certificates/extracted/cadir/ISRG_Root_X1.pem /etc/ca-certificates/extracted/cadir/ISRG_Root_X2.pem
wc -c /etc/ca-certificates/extracted/cadir/GTS_Root_R4.pem /etc/ca-certificates/extracted/cadir/ISRG_Root_X1.pem /etc/ca-certificates/extracted/cadir/ISRG_Root_X2.pem
```

Salida literal de `sha256sum`:

```
7e8b80d078d3dd77d3ed2108dd2b33412c12d7d72cb0965741c70708691776a2  /etc/ca-certificates/extracted/cadir/GTS_Root_R4.pem
22b557a27055b33606b6559f37703928d3e4ad79f110b407d04986e1843543d1  /etc/ca-certificates/extracted/cadir/ISRG_Root_X1.pem
a13d881e11fe6df181b53841f9fa738a2d7ca9ae7be3d53c866f722b4242b013  /etc/ca-certificates/extracted/cadir/ISRG_Root_X2.pem
```

Tamaños: 765, 1939 y 790 bytes. El bundle antiguo era la concatenación de esos tres ficheros, sin
líneas `#`: 3494 B, sha256 `c52dbce40b49e83f750f697796b24aee2dd039fe736b75bfb25aa5c18408dade`.
Cabe en el tope de 64 KiB de `wisp.js` (`MAX_CAS_BYTES`): 3487 B son el 5,32 % y dejan 62049 B de
margen. El bundle mínimo R4+X1 (2704 B en la medición p11-kit) no se elige: fijar su huella
exigiría re-medirlo. El `ca-certificates.crt` completo (172240 B, 168845 B normalizado, 113 certs
y líneas `#`) no cabe y no se versiona.

`studio/cas-wisp.js` exporta el bundle de curl como `CAS_WISP_PEM` y fija `CAS_WISP_SHA256`. La
comprobación sobre el propio módulo:

```sh
cd studio && node --input-type=module -e "import {CAS_WISP_PEM,CAS_WISP_SHA256} from './cas-wisp.js'; import {createHash} from 'node:crypto'; const b=new TextEncoder().encode(CAS_WISP_PEM); console.log(createHash('sha256').update(b).digest('hex')); console.log(CAS_WISP_SHA256); console.log(b.length);"
```

Salida literal:

```
c2b67c490da9b82d68b77f88c924288faa78fcc66077088c530921053ff19a2c
c2b67c490da9b82d68b77f88c924288faa78fcc66077088c530921053ff19a2c
3487
```

### Lo que se implementó y cómo se falsa

`studio/wisp.js` importa las dos constantes y, si no hay `File` local, llama a
`prepararCAS({nombre:"cas-wisp.pem", bytes:new TextEncoder().encode(CAS_WISP_PEM),
huellaEsperada:CAS_WISP_SHA256})` desde `prepararCASOficial()`. El nombre es lógico y pasa
`nombreCASValido`/`EXTENSION_CAS`: no hay fichero ni petición. El payload `{pem, huella}` se entrega
a `cargarCAsEnNucleo` **antes** de `obtener`; si la huella o el puente fallan, `SIN_CAS` avisa en
`#wisp-cas-estado` y no se abre ningún socket. Sin MIME nuevo, sin entrada en `sw.js` ni en
`servidor.js`, sin `fetch`: `git archive` y `tools/sitio.sh` empaquetan el módulo solos.

```sh
cd studio && node --check wisp.js && node --check cas-wisp.js && node --check sw.js && node --check servidor.js && node --check deps.js && node --check prueba.js && node prueba.js
```

Los `node --check` salen sin mensaje (código 0). La suite termina con `prueba: todo en orden`,
código de salida 0, sin `PRUEBA FALLA`. La falsación de un byte del PEM (`MIICCTCC` → `MIICCTCD`)
con `huellaEsperada: CAS_WISP_SHA256` se rechaza con `no coincide`, y la prueba además comprueba
que el núcleo recibe exactamente `{pem, huella}` del bundle y que el orden es `cas,obtener`.
`prueba.js:5392-5395` sigue exigiendo las siete frases del aviso en el HTML pintado, ahora con
`repo1.maven.org` y sin el sentido viejo («se lee de tu dispositivo» / «nunca del relay»).

### Confianza auditable y licencia

La confianza es **auditable-en-git**: no la absuelve una autoridad externa ni un relay. Quien lea
el sitio puede ver el historial y ejecutar `git diff -- studio/cas-wisp.js` para revisar el cambio
de CA y el commit que fijó `CAS_WISP_SHA256`. El aviso lo dice y el bundle lleva huella fija: un
cambio superficial rompe la prueba antes de publicar.

Texto literal de la licencia, sin inventarla:

```
Raíces GTS Root R4 / ISRG Root X1 / ISRG Root X2 desde https://curl.se/ca/cacert.pem (Mozilla CA store, MPL 2.0 según curl.se/docs/caextract.html, sección «CA certificate store license»); bundle de curl con fecha 2026-11-05, sha256 427c064216761a1e72a8b30f0d954d08cb6e4435a82091fb865afd20c8497285; subconjunto incrustado de 3487 bytes, sha256 c2b67c490da9b82d68b77f88c924288faa78fcc66077088c530921053ff19a2c.
```

**Confirmación escrita de licencia (2026-11-24, vía web, licencia libre):** la sección «CA certificate
store license» de `https://curl.se/docs/caextract.html` dice literalmente: «The PEM file is only a
converted version of the original one and thus it is licensed under the same license as the Mozilla
source file: MPL 2.0». La «Mozilla Root Store Policy» v3.1 (vigente desde 2026-07-01),
`https://www.mozilla.org/en-US/about/governance/policies/security-group/certs/policy/`, sección 1,
dice: «under the terms of the relevant Mozilla license(s), distributors of such software are
permitted to add or delete CA certificates and modify the values of the trust bits in the versions
that they distribute». Ambas son licencias libres: MPL 2.0 es una licencia de código abierto, y la
política de Mozilla permite redistribuir el store. Con esto se levanta el bloqueo de publicación y
`git push` de `apkc-studio` que imponía este §349; la huella `CAS_WISP_SHA256` sigue fijada y cualquier
cambio del bundle queda en el historial de git.

**Alcance exacto de la confirmación**: la frase literal de MPL 2.0 de curl se refiere a su propio
`cacert.pem` convertido, y ahora ese fichero es literalmente el origen del que se extrajeron los
tres bloques: ya no queda la salvedad de PEM sin licencia adjunta. La Root Store Policy v3.1 sigue
cubriendo la redistribución del store. **Uso real verificado**: `cas-wisp.js` se importa en
`studio/wisp.js:13`; `prepararCASOficial()` (`:411-415`) entrega `{pem,huella}` a `cargarCAsEnNucleo`
(`:426-447`) antes de `obtener`, y el puente Go lo guarda en `apkc/puente_wisp_js.go:171-193` y lo
usa como `CertPool` en `:134-138,164`. Por defecto WISP está desactivado (`deps-index.json`); solo
se usan estos PEM al activar `localStorage["apkc-studio/wisp"]`. El push a `apkc-studio` se ejecutó
(`e9274dd..a4e983c`); este cambio se commitea en el submódulo y se actualiza su puntero aquí, sin
push, en esta corrida.

En la medición original, `ls -ld /usr/share/doc/ca-certificates/` daba
`ls: cannot access '/usr/share/doc/ca-certificates/': No such file or directory`; el `README`
local de `/etc/ca-certificates/` tiene 4 líneas (`README:1-4`) y solo remite a
`update-ca-trust(8)`, sin licencia ni copyright. Esa era la razón de la salvedad retirada: la
procedencia ya no depende de p11-kit sino del `cacert.pem` de curl. `surge publish` sigue pendiente
de autorización expresa del usuario, no de licencia. Sin gate, CI ni chromium en esta corrida.
