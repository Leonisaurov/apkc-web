package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func servidorPrueba(t *testing.T) (*httptest.Server, string) {
	return servidorPruebaCon(t, politicaPrueba, 64)
}

func servidorPruebaProduccion(t *testing.T) (*httptest.Server, string) {
	return servidorPruebaCon(t, politicaProduccion, 64)
}

func servidorPruebaCon(t *testing.T, destinos politicaDestinos, credito uint32) (*httptest.Server, string) {
	t.Helper()
	allow, err := nuevaAllowlist(false, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv := nuevoServidor(allow, credito, 8)
	srv.destinos = destinos
	s := httptest.NewServer(srv)
	t.Cleanup(s.Close)
	return s, s.URL
}

func ecoTCP(t *testing.T) (string, uint16) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()
	return "127.0.0.1", uint16(ln.Addr().(*net.TCPAddr).Port)
}

func ecoUDP(t *testing.T) (string, uint16) {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	go func() {
		buf := make([]byte, tamMaxDatagrama)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_, _ = conn.WriteToUDP(buf[:n], addr)
		}
	}()
	return "127.0.0.1", uint16(conn.LocalAddr().(*net.UDPAddr).Port)
}

// Router una peticion WS con un Origin dado y devuelve la respuesta del
// handshake. El cliente la cierra en cuanto la ha leido.
func intentarHandshake(t *testing.T, urlBase, origen string) (*http.Response, error) {
	t.Helper()
	cabeceras := http.Header{}
	if origen != "" {
		cabeceras.Set("Origin", origen)
	}
	ws, resp, err := (&websocket.Dialer{HandshakeTimeout: 3 * time.Second}).
		Dial(urlWS(urlBase)+"/wisp/", cabeceras)
	if ws != nil {
		_ = ws.Close()
	}
	return resp, err
}

func TestHandshakeInicial(t *testing.T) {
	_, url := servidorPrueba(t)
	cl := abrirCliente(t, url, origenProduccion)
	p := cl.leer()
	if p.tipo != tipoContinue || p.streamID != 0 {
		t.Fatalf("CONTINUE inicial: tipo=0x%02x streamID=%d, quería 0x03 y 0", p.tipo, p.streamID)
	}
	if creditoDe(p) == 0 {
		t.Fatalf("CONTINUE inicial con crédito 0")
	}
}

func TestOrigenAjenoRechazado(t *testing.T) {
	s, url := servidorPrueba(t)
	defer s.Close()
	resp, err := intentarHandshake(t, url, "https://mal.example")
	if err == nil {
		t.Fatalf("origen ajeno aceptado: resp=%v", resp)
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("origen ajeno: resp=%v err=%v, quería 403", resp, err)
	}
}

func TestOrigenAutorizado(t *testing.T) {
	s, url := servidorPrueba(t)
	defer s.Close()
	resp, err := intentarHandshake(t, url, origenProduccion)
	if err != nil {
		t.Fatalf("origen de producción rechazado: resp=%v err=%v", resp, err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("origen de producción: status=%d, quería 101", resp.StatusCode)
	}
	cl := abrirCliente(t, url, origenProduccion)
	if p := cl.leer(); p.tipo != tipoContinue || p.streamID != 0 {
		t.Fatalf("tras 101: paquete 0x%02x en stream %d", p.tipo, p.streamID)
	}
}

func TestOrigenNullRechazado(t *testing.T) {
	s, url := servidorPrueba(t)
	defer s.Close()
	for _, origen := range []string{"null", ""} {
		resp, err := intentarHandshake(t, url, origen)
		if err == nil {
			t.Fatalf("origen %q aceptado: resp=%v", origen, resp)
		}
		if resp == nil || resp.StatusCode != http.StatusForbidden {
			t.Fatalf("origen %q: resp=%v err=%v, quería 403", origen, resp, err)
		}
	}
}

func TestEchoTCPYCierreLimpio(t *testing.T) {
	_, url := servidorPrueba(t)
	host, puerto := ecoTCP(t)
	cl := abrirCliente(t, url, origenProduccion)
	cl.leer() // CONTINUE inicial

	const id uint32 = 1
	cl.connect(id, tipoFlujoTCP, host, puerto)
	mensaje := []byte("hola wisp tcp")
	cl.enviar(tipoData, id, mensaje)

	p := cl.leer()
	if p.tipo != tipoData || p.streamID != id || string(p.carga) != string(mensaje) {
		t.Fatalf("eco TCP: tipo=0x%02x stream=%d carga=%q", p.tipo, p.streamID, p.carga)
	}

	cl.enviar(tipoClose, id, []byte{motivoCierreVoluntario})
	p = cl.leer()
	if p.tipo != tipoClose || p.streamID != id {
		t.Fatalf("cierre TCP: tipo=0x%02x stream=%d", p.tipo, p.streamID)
	}
}

func TestEchoUDP(t *testing.T) {
	_, url := servidorPrueba(t)
	host, puerto := ecoUDP(t)
	cl := abrirCliente(t, url, origenProduccion)
	cl.leer() // CONTINUE inicial

	const id uint32 = 7
	cl.connect(id, tipoFlujoUDP, host, puerto)
	mensaje := make([]byte, 1024)
	for i := range mensaje {
		mensaje[i] = byte(i)
	}
	cl.enviar(tipoData, id, mensaje)

	p := cl.leer()
	if p.tipo != tipoData || p.streamID != id {
		t.Fatalf("eco UDP: tipo=0x%02x stream=%d, quería DATA en %d", p.tipo, p.streamID, id)
	}
	if len(p.carga) > tamMaxDatagrama || string(p.carga) != string(mensaje) {
		t.Fatalf("datagrama UDP: longitud=%d, contenido distinto", len(p.carga))
	}

	// En UDP no se envía CONTINUE para el flujo; solo el inicial de streamID=0.
	_ = cl.ws.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	_, mensajeExtra, err := cl.ws.ReadMessage()
	if err == nil {
		p, _, errDec := decodificarPaquete(mensajeExtra)
		if errDec == nil && p.tipo == tipoContinue && p.streamID == id {
			t.Fatalf("CONTINUE inesperado en flujo UDP %d", id)
		}
	}
}

func TestLimiteFlujos(t *testing.T) {
	_, url := servidorPrueba(t)
	host, puerto := ecoTCP(t)
	cl := abrirCliente(t, url, origenProduccion)
	cl.leer() // CONTINUE inicial

	for id := uint32(1); id <= 8; id++ {
		cl.connect(id, tipoFlujoTCP, host, puerto)
	}
	cl.connect(9, tipoFlujoTCP, host, puerto)

	for i := 0; i < 8; i++ {
		p := cl.leer()
		if p.tipo == tipoClose && p.streamID == 9 && p.carga[0] == motivoLimitado {
			return
		}
	}
	t.Fatal("el noveno flujo no recibió CLOSE 0x49")
}

func TestLimiteConexiones(t *testing.T) {
	s, url := servidorPrueba(t)
	defer s.Close()

	for i := 0; i < maxConexiones; i++ {
		cl := abrirCliente(t, url, origenProduccion)
		cl.leer() // CONTINUE inicial
	}

	cabeceras := http.Header{}
	cabeceras.Set("Origin", origenProduccion)
	ws, _, err := (&websocket.Dialer{HandshakeTimeout: 3 * time.Second}).
		Dial(urlWS(url)+"/wisp/", cabeceras)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	_ = ws.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, mensaje, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("leyendo CLOSE de la conexión 17: %v", err)
	}
	p, _, err := decodificarPaquete(mensaje)
	if err != nil {
		t.Fatal(err)
	}
	if p.tipo != tipoClose || p.streamID != 0 || p.carga[0] != motivoLimitado {
		t.Fatalf("conexión 17: tipo=0x%02x stream=%d motivo=0x%02x, quería CLOSE 0x49 en 0",
			p.tipo, p.streamID, p.carga[0])
	}
}

func TestAllowlistNormaliza(t *testing.T) {
	a, err := nuevaAllowlist(false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !a.permite("HTTPS://APKc-Studio.Surge.SH/") {
		t.Fatal("la normalización en minúsculas rechazó el origen de producción")
	}
	if a.permite("https://apkc-studio.surge.sh/ruta") {
		t.Fatal("se aceptó un origen con ruta")
	}
	if a.permite("wss://wisp.dolci-vita.link") {
		t.Fatal("el servidor entró en la allowlist")
	}
	if a.permite("http://127.0.0.1:8801") {
		t.Fatal("el origen de desarrollo se aceptó sin -dev")
	}
	dev, err := nuevaAllowlist(true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !dev.permite("http://127.0.0.1:8801") {
		t.Fatal("-dev no habilitó el origen local")
	}
	if !dev.permite(origenProduccion) {
		t.Fatal("-dev retiró el origen de producción")
	}
}
