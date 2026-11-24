package main

// Transporte WISP en memoria para los tests nativos.
//
// Cada extremo entrega mensajes completos al otro, como haría un WebSocket
// binario: `Enviar` deposita un mensaje y `Recibir` lo devuelve entero. No hay
// red, ni TLS del navegador, ni `syscall/js`, así que el framing y el control de
// flujo se prueban con `go test` a secas.

import (
	"net"
	"sync"
)

// extremoMemoria es un lado del par. El canal tiene algo de holgura para que el
// emisor no se bloquee en cada mensaje, pero conserva la presión de un socket.
type extremoMemoria struct {
	entrada chan []byte
	otro    *extremoMemoria
	cerrada chan struct{}
	unaVez  sync.Once
}

// nuevoParMemoria devuelve los dos extremos conectados.
func nuevoParMemoria() (*extremoMemoria, *extremoMemoria) {
	a := &extremoMemoria{entrada: make(chan []byte, 64), cerrada: make(chan struct{})}
	b := &extremoMemoria{entrada: make(chan []byte, 64), cerrada: make(chan struct{})}
	a.otro = b
	b.otro = a
	return a, b
}

func (e *extremoMemoria) Enviar(b []byte) error {
	select {
	case <-e.cerrada:
		return net.ErrClosed
	default:
	}
	copia := make([]byte, len(b))
	copy(copia, b)
	select {
	case e.otro.entrada <- copia:
		return nil
	case <-e.otro.cerrada:
		return net.ErrClosed
	case <-e.cerrada:
		return net.ErrClosed
	}
}

func (e *extremoMemoria) Recibir() ([]byte, error) {
	select {
	case mensaje := <-e.entrada:
		return mensaje, nil
	case <-e.cerrada:
		return nil, net.ErrClosed
	}
}

func (e *extremoMemoria) Cerrar() error {
	e.unaVez.Do(func() {
		close(e.cerrada)
	})
	return nil
}
