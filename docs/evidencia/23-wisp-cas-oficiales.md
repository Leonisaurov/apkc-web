## 349. CAs oficiales del propio origen para el módulo WISP, con huella fija

Medida el 2026-11-24 en el worktree `wisp-cas-oficiales-impl`, sobre el submódulo `studio/`
(commit `bd99a0b06880d66313ab40a929f116dd874fdfa5`), sin chromium, sin CDP, sin gate, sin red,
sin `git push` y sin publicar. Cierra el plan aprobado
`…/plan-wisp-cas-oficiales/PLAN-wisp-cas-oficiales.md` de la sesión `ses_def6aa358ffe52VV0dDnWRezPH`.
Revierte la forma «solo `File` local» de `§257`/`§258`: las CAs oficiales de los dos hosts
autorizados (`apkc/depswisp.go:90-93`) entran como bundle incrustado en el propio origen; el `File`
local sigue como override. El módulo **sigue opt-in y apagado por defecto**; el relay jamás es
fuente de confianza y no hay `fetch` de CAs.

### La medición TLS previa (no se reabre aquí)

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
  sobre el toolchain fijado. Aquí se registra ese resultado, no se vuelve a lanzar: no hay red en
  esta corrida y la temperatura está alta.

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

### Los tres roots y el bundle

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

Tamaños: 765, 1939 y 790 bytes. El bundle es la concatenación de los tres ficheros, sin líneas
`#` (`grep -c '^#'` da `0`, código 1):

```sh
mkdir -p "$HOME/.cache/apkc-web/cas-wisp-impl"
cat /etc/ca-certificates/extracted/cadir/GTS_Root_R4.pem /etc/ca-certificates/extracted/cadir/ISRG_Root_X1.pem /etc/ca-certificates/extracted/cadir/ISRG_Root_X2.pem > "$HOME/.cache/apkc-web/cas-wisp-impl/bundle.pem"
wc -c "$HOME/.cache/apkc-web/cas-wisp-impl/bundle.pem"
sha256sum "$HOME/.cache/apkc-web/cas-wisp-impl/bundle.pem"
grep -c '^#' "$HOME/.cache/apkc-web/cas-wisp-impl/bundle.pem"
```

Salida literal:

```
3494 /home/leonisauro/.cache/apkc-web/cas-wisp-impl/bundle.pem
c52dbce40b49e83f750f697796b24aee2dd039fe736b75bfb25aa5c18408dade  /home/leonisauro/.cache/apkc-web/cas-wisp-impl/bundle.pem
0
```

Cabe en el tope de 64 KiB de `wisp.js` (`MAX_CAS_BYTES`): 3494 B son el 5,33 % y dejan 62042 B
de margen. El bundle mínimo R4+X1 (2704 B, huella truncada en la medición) no se elige: fijar su
huella exigiría re-medirla. El `ca-certificates.crt` completo (172240 B, 168845 B normalizado,
113 certs y líneas `#`) no cabe y no se versiona.

`studio/cas-wisp.js` exporta ese mismo texto como `CAS_WISP_PEM` y fija `CAS_WISP_SHA256`. La
comprobación sobre el propio módulo:

```sh
cd studio && node --input-type=module -e "import {CAS_WISP_PEM,CAS_WISP_SHA256} from './cas-wisp.js'; import {createHash} from 'node:crypto'; const b=new TextEncoder().encode(CAS_WISP_PEM); console.log(createHash('sha256').update(b).digest('hex')); console.log(CAS_WISP_SHA256); console.log(b.length);"
```

Salida literal:

```
c52dbce40b49e83f750f697796b24aee2dd039fe736b75bfb25aa5c18408dade
c52dbce40b49e83f750f697796b24aee2dd039fe736b75bfb25aa5c18408dade
3494
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
Raíces GTS_Root_R4 / ISRG_Root_X1 / ISRG_Root_X2 desde /etc/ca-certificates/extracted/cadir/ (p11-kit-trust, Mozilla CA store); texto público, licencia NO verificada localmente — confirmar MPL 2.0 / Mozilla CA Certificate Policy antes de publicar.
```

Fuentes ausentes: `ls -ld /usr/share/doc/ca-certificates/` da
`ls: cannot access '/usr/share/doc/ca-certificates/': No such file or directory`; el `README`
local de `/etc/ca-certificates/` tiene 4 líneas (`README:1-4`) y solo remite a
`update-ca-trust(8)`, sin licencia ni copyright. La fuente declarada de los PEM es
`mozilla.trust.p11-kit`. **No publicar** (`surge publish`, `git push` a `apkc-studio`) hasta
confirmar por escrito MPL 2.0 y la Mozilla CA Certificate Policy y anotarlo aquí. Sin gate, CI,
chromium ni red en esta corrida.
