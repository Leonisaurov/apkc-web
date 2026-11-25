package main

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Cliente WebSocket usado solo en las pruebas: habla el mismo formato de
// paquetes que el servidor y no comparte su lógica de relay.
type clientePrueba struct {
	t     *testing.T
	ws    *websocket.Conn
	resto []byte // paquetes coalescidos del último mensaje leído
}

func urlWS(urlHTTP string) string {
	if strings.HasPrefix(urlHTTP, "https://") {
		return "wss://" + strings.TrimPrefix(urlHTTP, "https://")
	}
	return "ws://" + strings.TrimPrefix(urlHTTP, "http://")
}

func abrirCliente(t *testing.T, urlBase, origen string) *clientePrueba {
	t.Helper()
	cabeceras := http.Header{}
	cabeceras.Set("Origin", origen)
	d := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	ws, resp, err := d.Dial(urlWS(urlBase)+"/wisp/", cabeceras)
	if err != nil {
		t.Fatalf("dial con Origin %q: resp=%v err=%v", origen, resp, err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		_ = ws.Close()
		t.Fatalf("Origin %q: status=%d, quería 101", origen, resp.StatusCode)
	}
	cl := &clientePrueba{t: t, ws: ws}
	t.Cleanup(func() { _ = ws.Close() })
	return cl
}

func (cl *clientePrueba) leer() paquete {
	cl.t.Helper()
	if len(cl.resto) == 0 {
		_ = cl.ws.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, mensaje, err := cl.ws.ReadMessage()
		if err != nil {
			cl.t.Fatalf("leer mensaje: %v", err)
		}
		cl.resto = mensaje
	}
	p, resto, err := decodificarPaquete(cl.resto)
	if err != nil {
		cl.t.Fatalf("decodificar mensaje: %v", err)
	}
	cl.resto = resto
	return p
}

func (cl *clientePrueba) enviar(tipo byte, id uint32, carga []byte) {
	cl.t.Helper()
	if err := cl.ws.WriteMessage(websocket.BinaryMessage,
		codificarPaquete(tipo, id, carga)); err != nil {
		cl.t.Fatalf("enviar paquete 0x%02x: %v", tipo, err)
	}
}

func (cl *clientePrueba) connect(id uint32, tipoFlujo byte, host string, puerto uint16) {
	cl.t.Helper()
	carga := make([]byte, 3+len(host))
	carga[0] = tipoFlujo
	carga[1] = byte(puerto)
	carga[2] = byte(puerto >> 8)
	copy(carga[3:], host)
	cl.enviar(tipoConnect, id, carga)
}
