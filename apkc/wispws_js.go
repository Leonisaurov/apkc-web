//go:build js && wasm

package main

// Transporte WISP sobre el WebSocket del navegador.
//
// `syscall/js` da la API del WebSocket, que ya hace el handshake HTTP y la
// negociación de frames; este fichero solo convierte los mensajes binarios en
// `[]byte` y los entrega al cliente. Los paquetes WISP viajan como mensajes
// binarios completos y el navegador conserva sus límites, así que el
// decodificador ve exactamente lo que envió el relay.
//
// El WebSocket se abre con `binaryType = "arraybuffer"`: sin eso, el navegador
// entrega `Blob` y habría que leerlo con `FileReader`, que es asíncrono de otra
// forma y complica el bucle de lectura.

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"syscall/js"
	"time"
)

// plazoAperturaWISP acota la espera del evento `open`.
const plazoAperturaWISP = 30 * time.Second

// umbralBufferWISP es el máximo de bytes que se dejan encolados en el propio
// WebSocket (`bufferedAmount`) antes de esperar a que drenen. `ws.send` nunca
// falla por tamaño: el navegador acepta el mensaje y lo guarda en memoria, así
// que sin esta espera un Write grande crece el heap de la pestaña sin que nadie
// lo acote. 1 MiB son 64 mensajes del tamaño máximo de DATA.
const umbralBufferWISP = 1 << 20

type transporteWebSocket struct {
	ws js.Value

	envioMu sync.Mutex

	colaMu sync.Mutex
	cola   [][]byte

	// fallos lleva los errores de un solo flujo (trama demasiado grande) al
	// bucle de lectura sin cerrar el WebSocket.
	fallos chan error

	mu      sync.Mutex
	err     error
	cerrada chan struct{}
	aviso   chan struct{}
	unaVez  sync.Once
}

// AbrirWebSocketWISP abre el relay de una configuración ya validada (opt-in
// incluido) y espera el evento `open`. Exige `ConfigWISP.Validar`, igual que
// `ConectarWISP`, para que no exista una vía que se salte el interruptor.
// Bloquea la gorutina que lo llama, pero no el bucle de eventos: el runtime de
// Go devuelve el control a JavaScript mientras espera, y los `js.Func` pueden
// correr.
func AbrirWebSocketWISP(cfg ConfigWISP) (TransporteWISP, error) {
	if err := cfg.Validar(); err != nil {
		return nil, err
	}
	direccion := cfg.Relay
	t := &transporteWebSocket{
		cerrada: make(chan struct{}),
		aviso:   make(chan struct{}, 1),
		fallos:  make(chan error, 8),
	}
	abierto := make(chan error, 1)

	onAbrir := js.FuncOf(func(js.Value, []js.Value) any {
		select {
		case abierto <- nil:
		default:
		}
		return nil
	})
	onError := js.FuncOf(func(js.Value, []js.Value) any {
		err := errors.New("wisp: error del WebSocket del relay")
		select {
		case abierto <- err:
		default:
		}
		t.cerrarCon(err)
		return nil
	})
	onMensaje := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		datos := args[0].Get("data")
		if datos.Type() != js.TypeObject {
			return nil
		}
		// El tamaño se mira sobre el ArrayBuffer, antes de crear el Uint8Array:
		// una trama de N bytes no puede costar N bytes para ser rechazada.
		if err := comprobarMensajeWISP(datos.Get("byteLength").Int()); err != nil {
			cabecera := t.cabeceraDe(datos)
			if flujo, ok := flujoDeTramaGrandeWISP(cabecera); ok {
				t.entregarFallo(&errorFlujoWISP{flujo: flujo, err: err})
			} else {
				t.cerrarCon(err)
			}
			return nil
		}
		arreglo := js.Global().Get("Uint8Array").New(datos)
		mensaje := make([]byte, arreglo.Get("length").Int())
		js.CopyBytesToGo(mensaje, arreglo)
		t.encolar(mensaje)
		return nil
	})
	onCerrar := js.FuncOf(func(js.Value, []js.Value) any {
		t.cerrarCon(errors.New("wisp: el WebSocket del relay se cerró"))
		return nil
	})

	ws := js.Global().Get("WebSocket").New(direccion)
	ws.Set("binaryType", "arraybuffer")
	ws.Set("onopen", onAbrir)
	ws.Set("onmessage", onMensaje)
	ws.Set("onclose", onCerrar)
	ws.Set("onerror", onError)
	t.ws = ws

	// Los `js.Func` no se liberan: viven mientras dure la conexión, y liberar el
	// de `close` desde dentro del propio evento no es seguro. Es una conexión por
	// sesión de descargas, no una por artefacto.
	select {
	case err := <-abierto:
		if err != nil {
			return nil, err
		}
		return t, nil
	case <-t.cerrada:
		return nil, t.error()
	case <-time.After(plazoAperturaWISP):
		err := errors.New("wisp: el WebSocket no abrió a tiempo")
		t.cerrarCon(err)
		return nil, err
	}
}

func (t *transporteWebSocket) encolar(mensaje []byte) {
	if len(mensaje) == 0 {
		return
	}
	t.colaMu.Lock()
	select {
	case <-t.cerrada:
		t.colaMu.Unlock()
		return
	default:
	}
	t.cola = append(t.cola, mensaje)
	t.colaMu.Unlock()
	select {
	case t.aviso <- struct{}{}:
	default:
	}
}

func (t *transporteWebSocket) Enviar(b []byte) error {
	return t.EnviarConPlazo(b, time.Time{})
}

// EnviarConPlazo manda un mensaje esperando a que `bufferedAmount` drene. `hasta`
// es el plazo de escritura del flujo y manda sobre `plazoDrenajeWISP`: un Write
// con deadline corto no puede quedarse 10 s esperando por paquete.
func (t *transporteWebSocket) EnviarConPlazo(b []byte, hasta time.Time) error {
	select {
	case <-t.cerrada:
		return t.error()
	default:
	}
	t.envioMu.Lock()
	defer t.envioMu.Unlock()
	if t.ws.IsUndefined() || t.ws.IsNull() {
		return net.ErrClosed
	}
	if t.ws.Get("readyState").Int() != 1 {
		return net.ErrClosed
	}
	if err := t.esperarDrenaje(hasta); err != nil {
		return err
	}
	arreglo := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(arreglo, b)
	return t.enviarSeguro(arreglo)
}

// cabeceraDe copia solo los primeros bytes del ArrayBuffer, para saber de qué
// flujo era una trama que se descarta sin convertirla entera.
func (t *transporteWebSocket) cabeceraDe(datos js.Value) []byte {
	largo := datos.Get("byteLength").Int()
	if largo > 1024 {
		largo = 1024
	}
	arreglo := js.Global().Get("Uint8Array").New(datos.Call("slice", 0, largo))
	copia := make([]byte, largo)
	js.CopyBytesToGo(copia, arreglo)
	return copia
}

// entregarFallo pasa al bucle de lectura el error de un flujo concreto sin
// cerrar el WebSocket.
func (t *transporteWebSocket) entregarFallo(err error) {
	select {
	case t.fallos <- err:
	default:
	}
}

// esperarDrenaje espera a que `bufferedAmount` baje del umbral. En wasm,
// `time.Sleep` devuelve el control al bucle de eventos de JavaScript, que es
// quien vacía el búfer del socket; no es una espera activa.
func (t *transporteWebSocket) esperarDrenaje(hasta time.Time) error {
	pendiente := func() (int, bool) {
		valor := t.ws.Get("bufferedAmount")
		if valor.IsUndefined() || valor.IsNull() {
			return 0, false
		}
		return int(valor.Float()), true
	}
	return esperarDrenajeWISP(pendiente, umbralBufferWISP, hasta, time.Now, time.Sleep)
}

// enviarSeguro convierte en error la excepción de JavaScript que lanza `send`
// cuando el socket se cierra entre la comprobación de `readyState` y el envío.
func (t *transporteWebSocket) enviarSeguro(arreglo js.Value) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("wisp: el WebSocket rechazó el mensaje: %v", r)
		}
	}()
	t.ws.Call("send", arreglo)
	return nil
}

func (t *transporteWebSocket) Recibir() ([]byte, error) {
	for {
		t.colaMu.Lock()
		if len(t.cola) > 0 {
			mensaje := t.cola[0]
			t.cola = t.cola[1:]
			t.colaMu.Unlock()
			return mensaje, nil
		}
		t.colaMu.Unlock()
		select {
		case <-t.aviso:
		case err := <-t.fallos:
			return nil, err
		case <-t.cerrada:
			return nil, t.error()
		}
	}
}

func (t *transporteWebSocket) Cerrar() error {
	t.cerrarCon(net.ErrClosed)
	return nil
}

func (t *transporteWebSocket) cerrarCon(err error) {
	t.unaVez.Do(func() {
		t.mu.Lock()
		t.err = err
		t.mu.Unlock()
		close(t.cerrada)
		if !t.ws.IsUndefined() && !t.ws.IsNull() {
			func() {
				defer func() { _ = recover() }()
				t.ws.Call("close")
			}()
		}
	})
}

func (t *transporteWebSocket) error() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.err != nil {
		return t.err
	}
	return net.ErrClosed
}
