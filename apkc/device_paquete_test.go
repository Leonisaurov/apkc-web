package main

// La validación de lo que acaba dentro de una orden de shell: `am start` y `pidof` reciben el
// paquete y la actividad, así que un carácter de shell convierte una orden en dos. La mitad que
// conduce `runAdb` en un proceso hijo vive en `device_paquete_proceso_test.go` con `//go:build !js`,
// porque en la pestaña no hay procesos que lanzar.

import (
	"fmt"
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
