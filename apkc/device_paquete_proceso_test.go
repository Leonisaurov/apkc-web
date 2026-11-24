//go:build !js

package main

// El rechazo de un manifiesto con basura de shell antes de pedirle nada al dispositivo se mide en un
// proceso hijo: `runAdb` termina con un fatal, así que la única forma de ver que no llegó al `shell`
// es mirar el código de salida y el registro del adb de mentira. Bajo `GOOS=js` no hay procesos
// hijos, así que la prueba va con este tag en vez de con un `t.Skip`, que dejaría la pestaña en verde
// sin medir. La mitad que sí corre en wasm es `TestValidaPaqueteYActividad`
// (`device_paquete_test.go`), que es donde vive la validación misma.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const manifiestoConMain = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="%s">
	<application>
		<activity android:name="%s">
			<intent-filter><action android:name="android.intent.action.MAIN"/></intent-filter>
		</activity>
	</application>
</manifest>`

// TestRunRechazaManifiestoInyectadoAntesDeLaShell conduce `runAdb` en un proceso hijo con un adb de
// mentira que apunta cada invocación. El manifiesto importado trae basura de shell y lo que se exige
// es que apkc salga con error, que el mensaje nombre el valor, y que no llegue a pedirle `shell` al
// dispositivo: quitar la validación deja las dos cosas en evidencia.
func TestRunRechazaManifiestoInyectadoAntesDeLaShell(t *testing.T) {
	if dir := os.Getenv("APKC_MANIFIESTO_INYECTADO"); dir != "" {
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}
		adbPath = os.Getenv("APKC_ADB_FALSO")
		runAdb()
		t.Fatal("runAdb no rechazó el manifiesto inyectado")
	}

	casos := []struct {
		nombre, pkg, actividad, culpable, motivo string
	}{
		{"paquete", "x; reboot", ".Main", "x; reboot", "paquete"},
		{"actividad", "com.x", ".Main; reboot", ".Main; reboot", "actividad"},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			dir := t.TempDir()
			manifiesto := fmt.Sprintf(manifiestoConMain, caso.pkg, caso.actividad)
			if err := os.WriteFile(filepath.Join(dir, "AndroidManifest.xml"), []byte(manifiesto), 0644); err != nil {
				t.Fatal(err)
			}

			registro := filepath.Join(dir, "adb.log")
			adbFalso := filepath.Join(dir, "adb")
			// contesta a `devices` con uno solo, y apunta cada llamada: así se ve si hubo `shell`
			guion := "#!/bin/sh\n" +
				"echo \"$@\" >> \"$APKC_ADB_REGISTRO\"\n" +
				"case \"$1\" in devices) printf 'List of devices attached\\nserial1\\tdevice\\n';; esac\n"
			if err := os.WriteFile(adbFalso, []byte(guion), 0755); err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command(os.Args[0], "-test.run=^TestRunRechazaManifiestoInyectadoAntesDeLaShell$")
			cmd.Env = append(os.Environ(),
				"APKC_MANIFIESTO_INYECTADO="+dir,
				"APKC_ADB_FALSO="+adbFalso,
				"APKC_ADB_REGISTRO="+registro)
			salida, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("runAdb no abortó con %s inyectado; salida:\n%s", caso.nombre, salida)
			}
			texto := string(salida)
			if !strings.Contains(texto, caso.culpable) || !strings.Contains(texto, caso.motivo) {
				t.Errorf("el rechazo no nombra %q como %s; salida:\n%s", caso.culpable, caso.motivo, texto)
			}

			llamadas, _ := os.ReadFile(registro)
			if strings.Contains(string(llamadas), "shell") {
				t.Errorf("apkc pidió `adb shell` con %s inyectado; llamadas:\n%s", caso.nombre, llamadas)
			}
			if !strings.Contains(string(llamadas), "install") {
				t.Errorf("el hijo no llegó ni a instalar, la prueba no midió el rechazo; llamadas:\n%s", llamadas)
			}
		})
	}
}
