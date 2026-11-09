# Informe: ejecutar apps Android desde Termux y desde el chroot

**Fecha de la medición:** 2026-11-09
**Aparato:** MEGA 3, Android 15 (SDK 35), kernel `5.10.226-android12`, aarch64
**Sesión medida:** Arch Linux ARM aarch64, `HOME=/home/leonisauro`, uid `10237`,
contexto SELinux `u:r:magisk:s0`, raíz `/data/data/com.termux/files/home/archlinux-chroot/rootfs`
(el fix de `enter.sh` ya está aplicado; el fallo de `am` sin entorno se midió antes del fix)
**Pregunta:** ¿por qué no se pueden ejecutar apps Android fácilmente; es culpa del chroot?

## Veredicto corto

**Sí y no.**

- La sesión **sí vive dentro del chroot de Arch Linux**. La raíz de `/proc/self/mountinfo` es
  `.../archlinux-chroot/rootfs`, `pwd` es `/termux/home/...` y `/etc/os-release` dice
  `Arch Linux ARM`. Ese chroot es el que explica que `cmd`, `input` y `screencap` se comporten
  distinto y que un `chroot` a pelo no tenga `/system`.
- **El fallo de `am` no es del chroot.** `am start` aborta con
  `BOOTCLASSPATH and DEX2OATBOOTCLASSPATH must not be empty` porque `app_process` no hereda el
  entorno de Android, no porque esté enjaulado. El mismo `am`, dentro del chroot, funciona en
  cuanto se copian esas dos variables del `environ` de `zygote64`; `enter.sh` ya lo hace desde el
  fix del 2026-11-09, y el bloque manual de este informe queda como fallback para sesiones
  anteriores.
- **Lo que sí es del chroot:** un `chroot` sin los montajes de `enter.sh` ve `/system`, `/apex` y
  `/vendor` como directorios vacíos y no tiene `am`, `cmd` ni `app_process`. Hay que entrar con el
  lanzador (o replicar sus `--bind`).
- **Lo que no es del chroot:** `cmd activity` se identifica como `com.android.shell` y el servicio
  de actividades lo rechaza por uid; `input` necesita `INJECT_EVENTS`. Eso es el modelo de
  permisos de Android, no el aislamiento del chroot.

La ruta que funciona para abrir apps y URLs es el `am` de Termux. En una sesión nueva de
`enter.sh` el `BOOTCLASSPATH` y el `DEX2OATBOOTCLASSPATH` ya están puestos; el bloque manual de
más abajo es el fallback para una sesión abierta antes del fix. El comando exacto que abrió Brave
Nightly está también en `docs/evidencia.md` §140.

## El entorno medido

```sh
id
cat /proc/self/attr/current
pwd
readlink /proc/self/root
awk '$5=="/"{print "root_mount="$4" fs="$9}' /proc/self/mountinfo
head -3 /etc/os-release
getprop ro.build.version.release
getprop ro.build.version.sdk
```

```text
uid=10237(leonisauro) gid=10237(leonisauro) groups=10237(leonisauro),983(video),995(audio),997(empower),998(wheel)
u:r:magisk:s0
/termux/home/Develop/Go/apkc-web
/
root_mount=/data/com.termux/files/home/archlinux-chroot/rootfs fs=/dev/block/dm-56
NAME="Arch Linux ARM"
PRETTY_NAME="Arch Linux ARM"
ID=archarm
15
35
```

Es decir: el proceso no corre en la raíz de Android. Corre en el rootfs de Arch, con `/termux`
como enlace a los ficheros de Termux. Por eso el repo se ve a la vez como
`/termux/home/Develop/Go/apkc-web` y como
`/data/data/com.termux/files/home/Develop/Go/apkc-web`: son el mismo inodo.

`enter.sh` monta, en un namespace de montajes privado, lo que un `chroot` normal no trae:

```text
/dev/block/dm-12 on /system      ext4  (ro)
/dev/block/dm-13 on /system_ext  ext4  (ro)
/dev/block/dm-14 on /vendor      ext4  (ro)
/dev/block/dm-56 on /data        f2fs
tmpfs            on /apex        tmpfs
binder           on /dev/binderfs binder
proc             on /proc        proc
sysfs            on /sys         sysfs
```

`enter.sh` crea además los enlaces `binder -> binderfs/binder`, `hwbinder` y `vndbinder`, y deja
`/system/bin` al final del `PATH`. Sin esos montajes, el `am` y el `cmd` de Android no existen
para el chroot.

## Matriz de comandos

| Comando | Desde | Resultado | Motivo |
| --- | --- | --- | --- |
| `am start ...` | Termux/chroot, sesión anterior al fix | aborta, rc 134 | `app_process` sin `BOOTCLASSPATH` ni `DEX2OATBOOTCLASSPATH` |
| `am start ...` | Termux/chroot, sesión nueva de `enter.sh` | funciona | `enter.sh` ya copia el entorno; `termux-am` llama a `startActivity` con el uid de Termux |
| `termux-open-url URL` | Termux/chroot, sesión nueva | funciona | el script usa el mismo `am` |
| `cmd package list packages` | Termux/chroot | funciona | el servicio de paquetes no comprueba el paquete llamante |
| `pm list packages` | Termux/chroot | `Failed transaction (2147483646)` | `pm` es un script `cmd package "$@"`; falla en esta sesión |
| `cmd package resolve-activity --user 0 ...` | Termux/chroot | funciona | lectura, con `--user 0` |
| `cmd activity start ...` | Termux/chroot | `Permission Denial: package=com.android.shell does not belong to uid=10237` | `cmd` se identifica como la shell de Android, no como Termux |
| `sudo cmd activity start-activity ...` | chroot | `Failed transaction (2147483646)` | el `cmd` de root no puede usar el servicio de actividades en este aparato |
| `sudo am start ...` | chroot | `Failed transaction (2147483646)` | `sudo` resuelve `/system/bin/am`, que usa `cmd activity`; no usa el `am` de Termux |
| `sudo input keyevent 3` | chroot | `Failed transaction (2147483646)` | el servicio de entrada rechaza al llamante |
| `input tap ...` | Termux/chroot | `SecurityException: INJECT_EVENTS` | Termux no tiene permiso de inyección |
| `screencap -p ...` | Termux/chroot | `Failed to take screenshot. Status: -1` | falta permiso de captura |
| `sudo screencap -p ...` | chroot | funciona | root sí puede capturar |
| `sudo dumpsys activity ...` | chroot | funciona | root puede leer el estado de actividades |
| `sudo /system/bin/service list` | chroot | funciona | root puede listar servicios binder |
| `sudo -u shell ...` | chroot | `unknown user shell` | el `sudo` del chroot no tiene usuario `shell` |
| `sudo chroot rootfs /bin/sh` | Termux/chroot | `/system`, `/apex`, `/vendor` vacíos; sin `am`/`cmd` | un chroot a pelo no hereda los `--bind` de `enter.sh` |

## Por qué falla `am` sin el entorno

El `am` de Termux (`/data/data/com.termux/files/usr/bin/am`) es un lanzador de `termux-am`:
ejecuta `/system/bin/app_process -Xnoimage-dex2oat / com.termux.termuxam.Am ...`. Ese
`app_process` necesita el entorno de la máquina virtual de Android. Sin él, ART aborta antes de
llegar al código Java:

```text
Check failed: !location.empty() BOOTCLASSPATH and DEX2OATBOOTCLASSPATH must not be empty
```

La solución es copiar esas dos variables del proceso `zygote64`, que es el que las tiene:

```sh
zygote=$(/usr/sbin/sudo sh -c 'pidof zygote64 2>/dev/null || pidof zygote 2>/dev/null' | tr -d '\r')
eval "$(/usr/sbin/sudo sh -c "tr '\0' '\n' < /proc/$zygote/environ" |
  grep -E '^(BOOTCLASSPATH|DEX2OATBOOTCLASSPATH)=' | sed 's/^/export /')"
```

Con eso, `am` funciona con el uid de Termux y sin `sudo`:

```sh
am start --user 0 \
  -n com.brave.browser_nightly/org.chromium.chrome.browser.ChromeTabbedActivity \
  -a android.intent.action.VIEW -d 'http://127.0.0.1:8787/'
```

Salida:

```text
Starting: Intent { act=android.intent.action.VIEW dat=http://127.0.0.1:8787/...
  cmp=com.brave.browser_nightly/org.chromium.chrome.browser.ChromeTabbedActivity }
```

Si la tarea ya existe, `am` responde
`Warning: Activity not started, its current task has been brought to the front`, pero el intento
`VIEW` llega y la pestaña abre la URL. Medido con una URL con `?t=$(date +%s)`: la barra de
direcciones la mostró y la página se pintó.

Para una app cualquiera, el mismo patrón con un componente explícito:

```sh
am start --user 0 -n com.android.settings/.Settings
```

Salida medida: `Starting: Intent { cmp=com.android.settings/.Settings }`, rc 0.

## Por qué `cmd` y `input` no son el camino

`cmd` es la herramienta de shell de Android, pensada para correr como el usuario `shell`
(uid 2000) y con el paquete `com.android.shell`. Dentro de Termux/chroot corre con uid 10237 y el
servicio de actividades comprueba la pareja paquete/uid:

```text
SecurityException: Permission Denial: package=com.android.shell does not belong to uid=10237
```

Como root el problema cambia, no desaparece:

```text
cmd: Failure calling service activity: Failed transaction (2147483646)
cmd: Failure calling service input: Failed transaction (2147483646)
```

El `sudo` del chroot no conoce un usuario `shell` (`sudo -u shell` -> `unknown user shell`), así
que no se puede bajar a uid 2000 por esa vía. `dumpsys`, `service list` y `screencap` sí funcionan
como root, pero no sirven para lanzar ni para inyectar toques.

Conclusión práctica: **no uses `sudo am` ni `sudo cmd` para abrir apps**. El `am` de Termux, como
usuario normal y con el entorno copiado, es la ruta que funciona.

## El chroot: qué es real y qué no

Lo que **sí** es del chroot:

1. La raíz es el rootfs de Arch, no la raíz de Android. `pwd` es `/termux/...` y `/etc/os-release`
   es Arch.
2. Un `chroot` a pelo no tiene `/system`, `/apex`, `/vendor` ni `app_process`; hay que usar
   `archlinux-chroot/enter.sh` o replicar sus `--bind`.
3. El namespace de montajes importa: si el chroot se creó antes de que `enter.sh` montara
   binderfs, `cmd`/`am` pueden abortar aunque `/system` exista. `~/ACCESO-ANDROID.md` documenta un
   caso así (namespace viejo, `/dev/binder` como archivo regular, `cmd package` con
   `Binder driver ... Permission denied`).

Lo que **no** es del chroot:

1. El aborto de `am` por `BOOTCLASSPATH`: ocurre igual fuera del chroot y se arregla copiando el
   entorno de `zygote64`.
2. La `Permission Denial` de `cmd activity`: es la comprobación paquete/uid de Android.
3. La falta de `INJECT_EVENTS` de `input`: es el permiso de Android, no el aislamiento.
4. El `Failed transaction` de `sudo cmd`/`sudo input`: es la combinación de root/contexto con esos
   servicios en este aparato. `sudo dumpsys` y `sudo service list` sí funcionan, así que no es que
   root no tenga binder: es que esos dos comandos de shell no aceptan a ese llamante.

## Cómo comprobar si estás en el chroot

Dentro de la sesión:

```sh
cat /etc/os-release                 # Arch Linux ARM
pwd                                 # /termux/...
readlink /proc/self/root            # /
awk '$5=="/"{print $4}' /proc/self/mountinfo
```

La última línea debe apuntar a `.../archlinux-chroot/rootfs`. Para comprobar que el namespace está
sano:

```sh
grep -c ' - binder binder ' /proc/self/mountinfo   # 1 en una sesión con binder
ls -l /dev/binder /dev/binderfs/binder             # symlink + char 234:1
ls -ld /system /apex /vendor                       # con contenido, no vacíos
```

En un `chroot` a pelo (sin `enter.sh`), `/system` y `/apex` son directorios vacíos y
`command -v am cmd` no encuentra nada. Eso es el síntoma de "el chroot no tiene Android".

## Recomendaciones

0. **En una sesión nueva de `enter.sh` no hay que exportar nada.** El lanzador copia
   `BOOTCLASSPATH` y `DEX2OATBOOTCLASSPATH` de `zygote64` y los deja en el entorno; `sudo` los
   conserva por `env_keep`. Se comprueba así:

   ```sh
   printf '%s\n' "${BOOTCLASSPATH:+BOOTCLASSPATH puesto}" \
                 "${DEX2OATBOOTCLASSPATH:+DEX2OATBOOTCLASSPATH puesto}"
   /usr/sbin/sudo sh -c 'echo "${BOOTCLASSPATH:+sudo tambien lo conserva}"'
   ```

1. **Abrir una URL** desde la sesión actual:

   ```sh
   termux-open-url 'http://127.0.0.1:8787/'
   ```

   `termux-open-url` usa el navegador predeterminado; en este aparato es Brave Nightly. Para
   forzar el paquete, usa el `am start -n ...` de más abajo.

2. **Abrir una app** con componente explícito:

   ```sh
   am start --user 0 -n com.android.settings/.Settings
   ```

   No hace falta `sudo`; el uid de Termux puede lanzar actividades exportadas. El mismo patrón
   sirve para `org.smart.connection/.MainActivity`.

3. **Fallback para una sesión anterior al fix** (o abierta fuera de `enter.sh`): copiar el
   entorno de `zygote64` a mano antes de usar `am`.

   ```sh
   zygote=$(/usr/sbin/sudo sh -c 'pidof zygote64 2>/dev/null || pidof zygote 2>/dev/null' | tr -d '\r')
   eval "$(/usr/sbin/sudo sh -c "tr '\0' '\n' < /proc/$zygote/environ" |
     grep -E '^(BOOTCLASSPATH|DEX2OATBOOTCLASSPATH)=' | sed 's/^/export /')"
   ```

4. **Leer el estado** (pantalla, actividades, servicios) con `sudo`:

   ```sh
   /usr/sbin/sudo dumpsys activity activities | grep -E 'topResumedActivity|mResumedActivity'
   /usr/sbin/sudo screencap -p /data/local/tmp/pantalla.png
   /usr/sbin/sudo /system/bin/service list | head
   ```

5. **No usar** `sudo am`, `sudo cmd` ni `sudo input` para lanzar o tocar. Si se necesita
   inyección de eventos, hace falta `INJECT_EVENTS` (Shizuku/`rish` o un mecanismo de root con el
   uid/contexto adecuado); este `sudo` no lo da.

6. **Si se entra a un chroot nuevo**, usar `archlinux-chroot/enter.sh`, no un `chroot` directo.
   El lanzador monta `/system`, `/system_ext`, `/vendor`, `/product`, `/odm`, `/apex`, `/data`,
   `/dev` con binderfs, `/proc` y `/sys`, y escribe `/run/android-native` con el estado de binder.

7. **Fijar el `HOME` de Termux** cuando un guion busque el SDK o el keystore:
   `HOME=/data/data/com.termux/files/home`. El `HOME` de esta sesión es `/home/leonisauro` (Arch)
   y no tiene `Android/`.

## Reproducción mínima

```sh
# 1. identidad y chroot
id
cat /proc/self/attr/current
awk '$5=="/"{print $4}' /proc/self/mountinfo

# 2. fallo historico sin entorno (solo en sesiones anteriores al fix)
am start -a android.intent.action.VIEW -d 'http://127.0.0.1:8787/'   # rc 134
logcat -d -t 80 | grep 'BOOTCLASSPATH and DEX2OATBOOTCLASSPATH'

# 3. en una sesion nueva de enter.sh esto ya viene puesto:
printf '%s\n' "${BOOTCLASSPATH:+BOOTCLASSPATH puesto}" \
              "${DEX2OATBOOTCLASSPATH:+DEX2OATBOOTCLASSPATH puesto}"

# 3b. fallback si la sesion es anterior al fix: copiar el entorno de zygote64
zygote=$(/usr/sbin/sudo sh -c 'pidof zygote64 2>/dev/null || pidof zygote 2>/dev/null' | tr -d '\r')
eval "$(/usr/sbin/sudo sh -c "tr '\0' '\n' < /proc/$zygote/environ" |
  grep -E '^(BOOTCLASSPATH|DEX2OATBOOTCLASSPATH)=' | sed 's/^/export /')"

# 4. abre Brave Nightly en la pestaña
am start --user 0 \
  -n com.brave.browser_nightly/org.chromium.chrome.browser.ChromeTabbedActivity \
  -a android.intent.action.VIEW -d 'http://127.0.0.1:8787/'

# 5. abre otra app
am start --user 0 -n com.android.settings/.Settings

# 6. lo que no funciona
cmd activity start --user 0 -p com.brave.browser -a android.intent.action.VIEW -d 'http://127.0.0.1:8787/'
input tap 10 10
/usr/sbin/sudo cmd activity start-activity -n com.android.settings/.Settings
/usr/sbin/sudo input keyevent 3
```

## Conclusión

El chroot es real y hay que tenerlo en cuenta: sin los montajes de `enter.sh` no hay `/system`,
`/apex`, `/vendor` ni `am`. Pero el problema que impedía abrir apps "fácilmente" no era el chroot
por sí solo: era el `BOOTCLASSPATH` que `app_process` no hereda. `enter.sh` ya lo copia desde
`zygote64`, así que en una sesión nueva el `am` de Termux lanza apps y URLs sin exportar nada a
mano; el bloque de copia manual queda para sesiones anteriores al fix. `cmd` e `input` siguen
fallando por uid/contexto y permisos de Android; para eso no hay atajo con este `sudo`, y el
informe lo deja medido para no volver a intentarlo a ciegas.

---

## Fix aplicado (verificado el 2026-11-09)

`enter.sh` copia automáticamente `BOOTCLASSPATH` y `DEX2OATBOOTCLASSPATH` del `environ` de
`zygote64` (líneas 197-207), los exporta al entrar en el chroot (líneas 476 y 493) y añade ambos
a `env_keep` del `sudoers` del chroot (línea 440), así que el usuario normal y root los conservan.

Comprobado en esta sesión, como uid 10237 y sin exportar nada a mano:

```sh
printf '%s\n' "${BOOTCLASSPATH:+BOOTCLASSPATH puesto}" \
              "${DEX2OATBOOTCLASSPATH:+DEX2OATBOOTCLASSPATH puesto}"
/usr/sbin/sudo sh -c 'echo "${BOOTCLASSPATH:+sudo tambien lo conserva}"'

am start --user 0 -n com.android.settings/.Settings
am start --user 0 -n org.smart.connection/.MainActivity
```

```text
BOOTCLASSPATH puesto
DEX2OATBOOTCLASSPATH puesto
sudo tambien lo conserva
Starting: Intent { cmp=com.android.settings/.Settings }
Starting: Intent { cmp=org.smart.connection/.MainActivity }
Warning: Activity not started, its current task has been brought to the front
```

`org.smart.connection` existe (`cmd package list packages` lo lista) y su `MainActivity` resolvió
con `cmd package resolve-activity --user 0 --brief -n org.smart.connection/.MainActivity`; la
segunda llamada trajo la tarea al frente y devolvió 0. `sudo` conserva las dos variables
(`len_boot=2624`, `len_dex=899`), pero `sudo am` sigue fallando con `Failed transaction` porque
resuelve el `/system/bin/am` de Android, no el `am` de Termux.

Lo que sigue funcionando como usuario: `cmd package list packages`, `cmd package path android`,
`logcat -d`, `wm size` y `getprop`. Lo que **no** funciona: `pm list packages`
(`cmd: Failure calling service package: Failed transaction (2147483646)`), `cmd activity start`
(`Permission Denial: package=com.android.shell does not belong to uid=10237`), `input keyevent`
(`INJECT_EVENTS`), y con `sudo`: `sudo am`, `sudo cmd activity` y `sudo input` (todos
`Failed transaction`). `sudo dumpsys activity`, `sudo screencap` y `sudo /system/bin/service list`
sí funcionan.

La ruta para abrir apps es el `am` de Termux con el entorno que ya pone `enter.sh`: no `sudo am`,
no `sudo cmd activity`, no `pm`.

---

## Fix shell uid (2026-11-09): el usuario existe, pero no arregla `cmd`/`input`

Se agregó el usuario `shell` (uid/gid 2000) y la regla
`leonisauro ALL=(shell) NOPASSWD:ALL` en el chroot. Comprobado:

```sh
getent passwd shell
# shell:x:2000:2000::/:/system/bin/sh
sudo -u shell id
# uid=2000(shell) gid=2000(shell) groups=2000(shell)
```

La regla está en `/etc/sudoers.d/90-leonisauro:4`. Pero en esta sesión el cambio de uid **no**
arregla el canal de shell de Android:

```sh
sudo -u shell cmd activity start-activity -n com.android.settings/.Settings
# cmd: Failure calling service activity: Failed transaction (2147483646)
sudo -u shell input keyevent 3
# cmd: Failure calling service input: Failed transaction (2147483646)
sudo -u shell pm list packages
# cmd: Failure calling service package: Failed transaction (2147483646)
sudo -u shell settings get secure default_input_method
# cmd: Failure calling service settings: Failed transaction (2147483646)
```

`sudo -u shell` cambia uid/gid, pero el contexto sigue siendo `u:r:magisk:s0` y `getenforce` dice
`Disabled`; no cambia el contexto ni el canal que usan `cmd`, `input`, `pm` y `settings`. Lo que
sí funciona como `shell`: `sudo -u shell dumpsys activity`, `sudo -u shell /system/bin/service
list` y `sudo -u shell /system/bin/screencap`. `cmd package list packages` sigue funcionando como
Termux (uid 10237); `pm list packages` no.

Se repitió en una sesión nueva lanzada con `enter.sh -c` (namespace nuevo): `id` da 2000, pero
`sudo -u shell cmd activity ...` sigue devolviendo `Failed transaction`. No es un namespace viejo:
el cambio de uid no basta.

Conclusión: el usuario `shell` y la regla de sudo se pueden dejar, pero **no son la ruta para
abrir apps ni para inyectar toques en esta sesión**. La ruta que funciona sigue siendo `am` como
Termux con el entorno de `enter.sh`; para `cmd`/`input` haría falta el contexto o el canal
adecuado (Shizuku, una sesión con el binder/shell correctos), no solo cambiar el uid.
