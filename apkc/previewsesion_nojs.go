//go:build !js || !wasm

package main

// En nativo no hay hospedador con globalThis.apkcSesion: la lógica de la sesión se prueba en
// previewsesion_test.go con un proveedor falso, sin este wrapper.

import (
	"fmt"
	"os"
)

func previewSesion() {
	fmt.Fprintln(os.Stderr, "preview --sesion solo existe en el binario wasm")
	os.Exit(2)
}
