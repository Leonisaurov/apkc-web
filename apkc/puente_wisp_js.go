//go:build js && wasm

package main

// El puente con la página, registrado en la instancia efímera de `js/wasm`.
//
// No hay demonio ni subcomando de servicio: `init()` solo publica dos globals. Abrir un
// WebSocket, montar TLS y descargar el artefacto solo ocurre dentro de
// `apkcWisp.obtener`, que la página llama después de que la persona acepte el
// aviso. Así el gate por defecto sigue viendo cero sockets y el modo externo
// sigue siendo un opt-in de extremo a extremo.
//
// El bundle de CAs y su huella se guardan en esta instancia; el `CertPool` se
// reconstruye en cada llamada desde esa pareja fijada, porque
// `CargarCAsPEMFijadas` es CPU pura. Nada de esto va a OPFS ni viaja por el relay.

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"syscall/js"
	"time"
)

var (
	muCAsPuente  sync.Mutex
	bundlePuente []byte
	huellaPuente [sha256.Size]byte
	poolPuente   *x509.CertPool
	casCargadas  bool
)

func init() {
	global := js.Global()
	global.Set("apkcWisp", map[string]any{
		"version": contratoPuenteWisp,
		"obtener": js.FuncOf(func(this js.Value, args []js.Value) (valor any) {
			// La red que impide que una excepción de `syscall/js` cruze el límite
			// y mate la instancia, igual que en `abrirWebSocketJS`.
			defer func() {
				if r := recover(); r != nil {
					valor = promesaRechazadaPuente(fmt.Sprintf("puente wisp (obtener): %v", r))
				}
			}()
			return promesaPuente("obtener", func() (any, error) {
				if len(args) == 0 {
					return nil, errors.New("puente wisp: obtener espera {version, relay, coordenada, destino}")
				}
				return obtenerPuente(args[0])
			})
		}),
	})
	global.Set("apkcWispCargarCAs", js.FuncOf(func(this js.Value, args []js.Value) (valor any) {
		defer func() {
			if r := recover(); r != nil {
				valor = promesaRechazadaPuente(fmt.Sprintf("puente wisp (cargarCAs): %v", r))
			}
		}()
		return promesaPuente("cargarCAs", func() (any, error) {
			if len(args) == 0 {
				return map[string]any{"ok": false, "error": "puente wisp: cargarCAs espera {pem, huella}"}, nil
			}
			return cargarCAsPuente(args[0])
		})
	}))
}

// promesaRechazadaPuente es la salida de emergencia cuando el propio registro
// del callback falla antes de poder crear la Promise con su executor.
func promesaRechazadaPuente(mensaje string) js.Value {
	return js.Global().Get("Promise").Call("reject", js.Global().Get("Error").New(mensaje))
}

// promesaPuente convierte un trabajo de Go en la Promise que espera la página.
// El trabajo va en una goroutine: `ConectarWISP` y el handshake necesitan que el
// bucle de eventos de JavaScript siga vivo para entregar los eventos del
// WebSocket. Ambos cachos llevan su propio `recover`, porque un pánico dentro de
// un callback de `js.FuncOf` no puede cruzar el límite de `syscall/js`.
func promesaPuente(nombre string, trabajo func() (any, error)) js.Value {
	return js.Global().Get("Promise").New(js.FuncOf(func(this js.Value, args []js.Value) any {
		resolver := args[0]
		rechazar := args[1]
		defer func() {
			if r := recover(); r != nil {
				rechazar.Invoke(js.Global().Get("Error").New(fmt.Sprintf("puente wisp (%s): %v", nombre, r)))
			}
		}()
		go func() {
			defer func() {
				if r := recover(); r != nil {
					rechazar.Invoke(js.Global().Get("Error").New(fmt.Sprintf("puente wisp (%s): %v", nombre, r)))
				}
			}()
			valor, err := trabajo()
			if err != nil {
				rechazar.Invoke(js.Global().Get("Error").New(err.Error()))
				return
			}
			resolver.Invoke(valor)
		}()
		return nil
	}))
}

// campoTextoPuente lee un campo del objeto que manda la página sin fiarse de que
// exista: un objeto ausente se trata como texto vacío y lo rechaza la validación.
func campoTextoPuente(objeto js.Value, clave string) string {
	if objeto.IsNull() || objeto.IsUndefined() || objeto.Type() != js.TypeObject {
		return ""
	}
	valor := objeto.Get(clave)
	if valor.IsNull() || valor.IsUndefined() {
		return ""
	}
	return valor.String()
}

// obtenerPuente es la mitad del contrato con `js.Value`. Toda la política vive
// en `validarPeticionPuente` y en el cliente WISP; aquí solo se junta el pool,
// se conecta y se convierte la respuesta al objeto de JavaScript.
func obtenerPuente(peticion js.Value) (any, error) {
	version := campoTextoPuente(peticion, "version")
	relay := campoTextoPuente(peticion, "relay")
	coord := campoTextoPuente(peticion, "coordenada")
	destino := campoTextoPuente(peticion, "destino")

	cfg, _, err := validarPeticionPuente(version, relay, coord, destino)
	if err != nil {
		return nil, err
	}
	pool, err := poolCAsPuente()
	if err != nil {
		return nil, err
	}
	cfg.RootCAs = pool

	cliente, err := ConectarWISP(cfg)
	if err != nil {
		return nil, err
	}
	conn, err := AbrirTLSWISP(cliente, hostPuenteWisp, puertoPuenteWisp, cfg)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	// `AbrirTLSWISP` limpia el deadline del handshake; el GET lleva el suyo.
	if err := conn.SetDeadline(time.Now().Add(plazoGetPuente)); err != nil {
		return nil, err
	}
	respuesta, err := obtenerConConn(conn, coord)
	if err != nil {
		return nil, err
	}

	arreglo := js.Global().Get("Uint8Array").New(len(respuesta.Bytes))
	js.CopyBytesToJS(arreglo, respuesta.Bytes)
	return map[string]any{
		"bytes":  arreglo,
		"sha256": respuesta.SHA256,
		"ruta":   respuesta.Ruta,
	}, nil
}

// poolCAsPuente reconstruye el almacén en cada llamada desde la pareja fijada.
// `CargarCAsPEMFijadas` no toca la red ni el sistema de ficheros: es solo CPU,
// así que no hay que compartir un `CertPool` mutable entre descargas.
func poolCAsPuente() (*x509.CertPool, error) {
	muCAsPuente.Lock()
	bundle := append([]byte(nil), bundlePuente...)
	huella := huellaPuente
	cargadas := casCargadas
	muCAsPuente.Unlock()
	if !cargadas {
		return nil, errors.New("puente wisp: no hay CAs cargadas; llama a apkcWispCargarCAs antes de obtener")
	}
	return CargarCAsPEMFijadas(bundle, huella)
}

// cargarCAsPuente comprueba la huella de 64 dígitos y el PEM antes de guardar la
// pareja en la instancia. Nunca llama a `PoolDeSistema`, que no existe en wasm.
// Los errores vuelven como `{ok:false, error}` porque el contrato de la página
// distingue un rechazo de CAs de un fallo del relay.
func cargarCAsPuente(peticion js.Value) (any, error) {
	pem := campoTextoPuente(peticion, "pem")
	hexHuella := campoTextoPuente(peticion, "huella")
	decodificada, err := hex.DecodeString(hexHuella)
	if err != nil || len(decodificada) != sha256.Size {
		return map[string]any{
			"ok": false,
			"error": fmt.Sprintf("puente wisp: la huella debe ser sha256 de 64 dígitos hexadecimales; recibida %q",
				hexHuella),
		}, nil
	}
	var huella [sha256.Size]byte
	copy(huella[:], decodificada)

	pool, err := CargarCAsPEMFijadas([]byte(pem), huella)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}, nil
	}
	muCAsPuente.Lock()
	bundlePuente = []byte(pem)
	huellaPuente = huella
	poolPuente = pool
	casCargadas = true
	muCAsPuente.Unlock()
	return map[string]any{"ok": true}, nil
}
