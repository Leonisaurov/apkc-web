package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidaPaqueteYActividad fija qué es un nombre utilizable: los dos valores acaban dentro de
// `am start -W -S -n <paquete>/<actividad>` y de `pidof -s <paquete>`, así que un carácter de shell
// convierte una orden en dos.
func TestValidaPaqueteYActividad(t *testing.T) {
	buenos := [][2]string{
		{"com.myapp", ".MainActivity"},
		{"com.hola.apkcstudio", "com.hola.apkcstudio.MainActivity"},
		{"com.x.y_2", "MainActivity"},
		{"com.x", ".Outer$Inner"},
	}
	for _, b := range buenos {
		if err := validarPaqueteYActividad(b[0], b[1]); err != nil {
			t.Errorf("validarPaqueteYActividad rechaza lo válido %q/%q: %v", b[0], b[1], err)
		}
	}

	malos := [][2]string{
		{"x; reboot", ".Main"},
		{"com.x;reboot", ".Main"},
		{"com.x", ".Main; reboot"},
		{"com.x", ".Main$(reboot)"},
		{"com x", ".Main"},
		{"com..x", ".Main"},
		{"com.x\nreboot", ".Main"},
	}
	for _, b := range malos {
		err := validarPaqueteYActividad(b[0], b[1])
		if err == nil {
			t.Errorf("validarPaqueteYActividad acepta %q/%q", b[0], b[1])
			continue
		}
		// el mensaje nombra el valor rechazado y el motivo, que es lo que necesita quien importa;
		// %q escapa el salto de línea, así que se busca también la forma citada
		culpable := b[0]
		if validarPaqueteYActividad(b[0], "Main") == nil {
			culpable = b[1]
		}
		if !strings.Contains(err.Error(), culpable) && !strings.Contains(err.Error(), fmt.Sprintf("%q", culpable)) {
			t.Errorf("el rechazo de %q/%q no nombra %q: %v", b[0], b[1], culpable, err)
		}
		if !strings.Contains(err.Error(), "paquete") && !strings.Contains(err.Error(), "actividad") {
			t.Errorf("el rechazo de %q/%q no dice qué está mal: %v", b[0], b[1], err)
		}
	}
}

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
