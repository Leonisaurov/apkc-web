//go:build !js

package main

// La mitad nativa de la sonda E: con una configuración ya validada, `AbrirWebSocketWISP` solo puede
// contestar que el WebSocket del navegador no existe con este `GOOS` (`wispws_nativo.go`). Es la
// aserción que impide que el aviso se confunda con un rechazo de `Validar`: si la validación
// fallara, el error diría «desactivado» y no «GOOS=js».
//
// Vive aparte porque bajo `GOOS=js` esa rama no existe —ahí `AbrirWebSocketWISP` abre el socket—, y
// la exclusión es por build tag y no por un `t.Skip`, que dejaría la suite en verde sin medir.

import (
	"strings"
	"testing"
)

// TestAbrirWebSocketWISPConConfigValidaAvisaDeNativo: en un binario nativo, una configuración válida
// no llega a abrir nada; el error lo dice.
func TestAbrirWebSocketWISPConConfigValidaAvisaDeNativo(t *testing.T) {
	cfg := NuevaConfigWISP()
	cfg.Habilitado = true
	cfg.Relay = RelaysWISP()[0]
	_, err := AbrirWebSocketWISP(cfg)
	if err == nil {
		t.Fatal("en nativo AbrirWebSocketWISP debía fallar tras validar")
	}
	if strings.Contains(err.Error(), "desactivado") || !strings.Contains(err.Error(), "GOOS=js") {
		t.Fatalf("la configuración válida no llegó al aviso de nativo: %v", err)
	}
}
