//go:build !js

package main

// Almacén de CAs para el TLS del cliente WISP fuera del navegador. En nativo se
// usa el del sistema; en `js/wasm` esa vía no existe y el llamador tiene que
// inyectar uno (ver `wispcerts_js.go`).

import (
	"crypto/x509"
	"fmt"
)

// PoolDeSistema devuelve una copia del almacén de CAs del sistema operativo.
func PoolDeSistema() (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("wisp: no se pudo leer el almacén de CAs del sistema: %w", err)
	}
	if pool == nil {
		return nil, fmt.Errorf("wisp: el sistema no ofrece un almacén de CAs")
	}
	return pool, nil
}
