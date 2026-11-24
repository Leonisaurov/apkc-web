//go:build js && wasm

package main

// La mitad de la pestaña de la sonda E: con una configuración ya validada, `AbrirWebSocketWISP`
// llega a `abrirWebSocketJS`, y lo que se mide es el rechazo de la dirección, no la red.
//
// El hospedador de Node publica un `WebSocket` global (undici): sin la guarda de abajo, esta prueba
// abriría una conexión de verdad al relay —una petición fuera del origen, que es justo lo que la
// regla 4 del repo prohíbe— y el veredicto dependería de que haya red. Con el global en `undefined`,
// el `recover` de `abrirWebSocketJS` convierte el fallo en error y la prueba mide lo que dice medir:
// que la configuración válida llega hasta el constructor y que su excepción no tumba la instancia.

import (
	"strings"
	"syscall/js"
	"testing"
)

// TestAbrirWebSocketWISPRechazaLaDireccionSinAbrirRed: con `WebSocket` ausente, el error nombra la
// dirección rechazada y el relay, y no es el aviso de `Validar`.
func TestAbrirWebSocketWISPRechazaLaDireccionSinAbrirRed(t *testing.T) {
	// se guarda el global para devolverlo pase lo que pase: el resto de la suite comparte instancia
	global := js.Global()
	guardado := global.Get("WebSocket")
	global.Set("WebSocket", js.Undefined())
	defer global.Set("WebSocket", guardado)

	// la validación sigue mandando: desactivado se rechaza antes de mirar el global
	if _, err := AbrirWebSocketWISP(NuevaConfigWISP()); err == nil || !strings.Contains(err.Error(), "desactivado") {
		t.Fatalf("con el módulo desactivado dio %v", err)
	}

	cfg := NuevaConfigWISP()
	cfg.Habilitado = true
	cfg.Relay = RelaysWISP()[0]
	_, err := AbrirWebSocketWISP(cfg)
	if err == nil {
		t.Fatal("sin WebSocket del navegador AbrirWebSocketWISP debía fallar")
	}
	if strings.Contains(err.Error(), "desactivado") {
		t.Fatalf("la configuración válida no pasó la validación: %v", err)
	}
	if !strings.Contains(err.Error(), "rechazó la dirección") || !strings.Contains(err.Error(), cfg.Relay) {
		t.Fatalf("el fallo no nombra la dirección rechazada ni el relay %q: %v", cfg.Relay, err)
	}
}
