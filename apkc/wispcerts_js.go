//go:build js

package main

// En `js/wasm` no hay almacén de CAs del sistema: `crypto/x509` compila con la
// lista de ficheros vacía (`root_wasm.go` en el GOROOT), así que `SystemCertPool`
// devuelve un pool vacío y cualquier handshake falla con «signed by unknown
// authority». La vía es inyectar los certificados desde la página con
// `ConfigWISP.RootCAs` (por ejemplo, el bundle que el usuario elija o el que
// traiga la interfaz). No se descarga ninguno en tiempo de ejecución.

import (
	"crypto/x509"
	"errors"
)

// PoolDeSistema no puede existir en el navegador y lo dice en vez de devolver un
// pool vacío que haría fallar el handshake más tarde y con peor mensaje.
func PoolDeSistema() (*x509.CertPool, error) {
	return nil, errors.New("wisp: js/wasm no tiene almacén de CAs del sistema; inyecta ConfigWISP.RootCAs con CargarCAsPEM")
}
