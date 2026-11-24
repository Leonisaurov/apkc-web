//go:build !js

package main

// En nativo no hay WebSocket del navegador. El cliente WISP se prueba con el
// transporte en memoria (`wispmemoria.go`) y, si algún día hace falta un ensayo
// real desde el escritorio, se añadirá un transporte concreto aquí en vez de
// arrastrar `syscall/js` a la compilación nativa.

import "errors"

// AbrirWebSocketWISP solo existe con GOOS=js GOARCH=wasm.
func AbrirWebSocketWISP(direccion string) (TransporteWISP, error) {
	return nil, errors.New("wisp: el WebSocket del navegador solo existe con GOOS=js GOARCH=wasm")
}
