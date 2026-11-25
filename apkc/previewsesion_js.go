//go:build js && wasm

package main

// El adaptador wasm del bucle de sesión. `siguiente()` del hospedador es síncrono y devuelve "" si
// no hay orden: entre sondeo y sondeo cedemos al event loop con setTimeout(...,0) + un canal, para
// que el worker pueda entregar la orden siguiente. APKC_SESION_OCUPADO=1 sustituye la cesión por un
// bucle ocupado y es la falsación que la sonda node tiene que rechazar con TIMEOUT.

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"syscall/js"
)

type ordenSesionJS struct {
	// el hospedador numera las órdenes con `++siguienteOrden` (hospedador.js:202), así que el id
	// llega como número; se conserva tal cual para que fin-orden devuelva la misma clave.
	ID   any      `json:"id"`
	Argv []string `json:"argv"`
}

func previewSesion() {
	set := flag.NewFlagSet("preview --sesion", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	idFlag := set.String("sesion", "", "id de la sesión viva del hospedador")
	var motivo string
	if err := set.Parse(os.Args[2:]); err != nil {
		motivo = err.Error()
	}
	rutas := set.Args()
	if motivo == "" && (len(rutas) != 1 || *idFlag == "") {
		motivo = "preview --sesion necesita --sesion <id> y la apk del arranque"
	}
	if motivo == "" && os.Getenv("APKC_SESION_ID") != *idFlag {
		motivo = "APKC_SESION_ID no coincide con --sesion"
	}
	var s *sesionPreview
	if motivo == "" {
		datos, err := os.ReadFile(rutas[0])
		if err != nil {
			motivo = err.Error()
		} else if s, err = nuevaSesionPreviewDelEntorno(datos, ""); err != nil {
			motivo = err.Error()
		}
	}
	if motivo != "" {
		fmt.Fprintln(os.Stderr, "preview --sesion:", motivo)
		os.Exit(2)
	}
	s.salir = os.Exit
	s.publica = func(id any, codigo int) {
		js.Global().Call("postMessage", map[string]any{
			"tipo": "apkc:fin-orden", "id": id, "codigo": codigo,
		})
	}
	bucleSesionJS(s)
}

func bucleSesionJS(s *sesionPreview) {
	ocupado := os.Getenv("APKC_SESION_OCUPADO") == "1"
	despierta := make(chan struct{}, 1)
	cb := js.FuncOf(func(this js.Value, args []js.Value) any {
		select {
		case despierta <- struct{}{}:
		default:
		}
		return nil
	})
	defer cb.Release()
	for {
		if !ocupado {
			js.Global().Call("setTimeout", cb, 0)
			<-despierta
		}
		crudo := js.Global().Get("apkcSesion").Call("siguiente").String()
		if crudo == "" {
			continue
		}
		var orden ordenSesionJS
		if err := json.Unmarshal([]byte(crudo), &orden); err != nil {
			continue
		}
		s.ejecutaOrden(orden.ID, orden.Argv, func(linea string) {
			fmt.Println(linea)
			// sondeo opcional: un hospedador puede pedir la línea sin capturar stdout
			host := js.Global().Get("apkcSesion")
			if !host.IsUndefined() && !host.IsNull() {
				aviso := host.Get("linea")
				if !aviso.IsUndefined() && !aviso.IsNull() {
					aviso.Invoke(linea)
				}
			}
		})
	}
}
