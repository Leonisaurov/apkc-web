//go:build !js

package main

// En nativo no hay WebSocket del navegador. El cliente WISP se prueba con el
// transporte en memoria (`wispmemoria.go`) y, si algún día hace falta un ensayo
// real desde el escritorio, se añadirá un transporte concreto aquí en vez de
// arrastrar `syscall/js` a la compilación nativa.

import "errors"

// AbrirWebSocketWISP solo existe con GOOS=js GOARCH=wasm. Valida la misma
// configuración que la versión del navegador, para que la comprobación del
// opt-in no dependa del build.
func AbrirWebSocketWISP(cfg ConfigWISP) (TransporteWISP, error) {
	if err := cfg.Validar(); err != nil {
		return nil, err
	}
	return nil, errors.New("wisp: el WebSocket del navegador solo existe con GOOS=js GOARCH=wasm")
}
