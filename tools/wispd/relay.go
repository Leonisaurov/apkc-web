package main

// Relay WISP v1 sobre un único WebSocket binario. Cada flujo se multiplexa por
// streamID y se atiende con dos goroutines: una lee del socket y encola DATA,
// otra saca los paquetes y los escribe en el WebSocket con un mutex compartido.
// Las cotas están pensadas para un VM con 956 MiB: la cola de cada flujo son 64
// paquetes de 16 KiB (1 MiB) y el global, 1024 paquetes (16 MiB).

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	maxConexiones   = 16
	cupoPorFlujo    = 64
	cupoGlobal      = (16 << 20) / tamMaxDatos
	tamMaxMensaje   = (1 << 20) + 5
	tamMaxDatagrama = 8 << 10

	plazoInactividad = 120 * time.Second
	plazoPing        = 30 * time.Second
	plazoPong        = 5 * time.Second
	plazoDial        = 5 * time.Second
)

type servidor struct {
	allow     *allowlist
	credito   uint32
	maxFlujos int

	// Semaforos: se envia para tomar hueco y se recibe para liberarlo.
	tokens     chan struct{}
	conexiones chan struct{}
}

func nuevoServidor(a *allowlist, credito uint32, maxFlujos int) *servidor {
	if credito == 0 {
		credito = 64
	}
	if maxFlujos < 1 {
		maxFlujos = 8
	}
	return &servidor{
		allow:      a,
		credito:    credito,
		maxFlujos:  maxFlujos,
		tokens:     make(chan struct{}, cupoGlobal),
		conexiones: make(chan struct{}, maxConexiones),
	}
}

func (s *servidor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// La URL del relay WISP termina siempre en barra.
	if !strings.HasSuffix(r.URL.Path, "/") {
		http.NotFound(w, r)
		return
	}
	origenes := r.Header.Values("Origin")
	if len(origenes) != 1 || !s.allow.permite(origenes[0]) {
		http.Error(w, "origen no autorizado", http.StatusForbidden)
		return
	}
	up := websocket.Upgrader{
		ReadBufferSize:  tamMaxDatos,
		WriteBufferSize: tamMaxDatos,
		CheckOrigin: func(req *http.Request) bool {
			return s.allow.permite(req.Header.Get("Origin"))
		},
	}
	ws, err := up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	select {
	case s.conexiones <- struct{}{}:
	default:
		_ = ws.WriteMessage(websocket.BinaryMessage,
			codificarPaquete(tipoClose, 0, []byte{motivoLimiteConexiones}))
		_ = ws.Close()
		return
	}
	defer func() { <-s.conexiones }()

	c := &conexion{
		srv:    s,
		ws:     ws,
		done:   make(chan struct{}),
		flujos: make(map[uint32]*flujo),
	}
	c.atender()
}

type conexion struct {
	srv  *servidor
	ws   *websocket.Conn
	done chan struct{}

	mu      sync.Mutex
	flujos  map[uint32]*flujo
	escribe sync.Mutex
	unaVez  sync.Once
}

type flujo struct {
	id     uint32
	tipo   byte
	conn   net.Conn
	salida chan []byte
	done   chan struct{}
	unaVez sync.Once
}

func (f *flujo) cerrarRed() {
	f.unaVez.Do(func() {
		close(f.done)
		if f.conn != nil {
			_ = f.conn.Close()
		}
	})
}

func (c *conexion) atender() {
	defer c.terminar()

	c.ws.SetReadLimit(tamMaxMensaje)
	c.actualizarPlazo()
	c.ws.SetPongHandler(func(string) error {
		c.actualizarPlazo()
		return nil
	})

	// CONTINUE inicial, antes de cualquier otra trama y con streamID=0.
	credito := make([]byte, 4)
	binary.LittleEndian.PutUint32(credito, c.srv.credito)
	if err := c.escribirPaquete(tipoContinue, 0, credito); err != nil {
		return
	}

	go c.latidos()
	for {
		_, mensaje, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		c.actualizarPlazo()
		for len(mensaje) > 0 {
			p, resto, err := decodificarPaquete(mensaje)
			if err != nil {
				return
			}
			mensaje = resto
			c.procesar(p)
		}
	}
}

func (c *conexion) actualizarPlazo() {
	_ = c.ws.SetReadDeadline(time.Now().Add(plazoInactividad))
}

// latidos envia ping cada 30 s; el pong del cliente renueva el plazo de lectura
// de 120 s. Sin trafico ni pong, ReadMessage expira y se cierra la conexion.
func (c *conexion) latidos() {
	ticker := time.NewTicker(plazoPing)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = c.ws.WriteControl(websocket.PingMessage, []byte("wispd"),
				time.Now().Add(plazoPong))
		case <-c.done:
			return
		}
	}
}

func (c *conexion) escribirPaquete(tipo byte, id uint32, carga []byte) error {
	select {
	case <-c.done:
		return errors.New("wispd: conexion cerrada")
	default:
	}
	c.escribe.Lock()
	defer c.escribe.Unlock()
	return c.ws.WriteMessage(websocket.BinaryMessage, codificarPaquete(tipo, id, carga))
}

func (c *conexion) procesar(p paquete) {
	switch p.tipo {
	case tipoConnect:
		c.abrirFlujo(p.streamID, p.carga)
	case tipoData:
		c.datosAlSocket(p.streamID, p.carga)
	case tipoContinue:
		// El credito lo fija este relay; el CONTINUE del cliente se ignora.
	case tipoClose:
		motivo := motivoDe(p)
		if motivo >= 0x41 && motivo <= 0x49 {
			motivo = motivoCierreVoluntario
		}
		c.cerrarFlujo(p.streamID, motivo, true)
	}
}

func (c *conexion) abrirFlujo(id uint32, carga []byte) {
	if id == 0 {
		_ = c.escribirPaquete(tipoClose, id, []byte{motivoCierreGenerico})
		return
	}
	sol, err := decodificarConnect(carga)
	if err != nil {
		_ = c.escribirPaquete(tipoClose, id, []byte{motivoCierreError})
		return
	}
	c.mu.Lock()
	if len(c.flujos) >= c.srv.maxFlujos {
		c.mu.Unlock()
		_ = c.escribirPaquete(tipoClose, id, []byte{motivoLimiteFlujos})
		return
	}
	if _, existe := c.flujos[id]; existe {
		c.mu.Unlock()
		_ = c.escribirPaquete(tipoClose, id, []byte{motivoCierreGenerico})
		return
	}
	f := &flujo{
		id:     id,
		tipo:   sol.tipoFlujo,
		salida: make(chan []byte, cupoPorFlujo),
		done:   make(chan struct{}),
	}
	c.flujos[id] = f
	c.mu.Unlock()

	dir := net.JoinHostPort(sol.host, strconv.Itoa(int(sol.puerto)))
	if sol.tipoFlujo == tipoFlujoUDP {
		ctx, cancel := context.WithTimeout(context.Background(), plazoDial)
		conn, errDial := (&net.Dialer{Timeout: plazoDial}).DialContext(ctx, "udp", dir)
		cancel()
		if errDial != nil {
			c.cerrarFlujo(id, motivoConexionRechazada, true)
			return
		}
		f.conn = conn
	} else {
		conn, errDial := net.DialTimeout("tcp", dir, plazoDial)
		if errDial != nil {
			c.cerrarFlujo(id, motivoConexionRechazada, true)
			return
		}
		f.conn = conn
	}

	go c.leerRed(f)
	go c.escribirRed(f)
}

func (c *conexion) datosAlSocket(id uint32, datos []byte) {
	c.mu.Lock()
	f := c.flujos[id]
	c.mu.Unlock()
	if f == nil {
		_ = c.escribirPaquete(tipoClose, id, []byte{motivoFlujoDesconocido})
		return
	}
	if f.tipo == tipoFlujoUDP && len(datos) > tamMaxDatagrama {
		c.cerrarFlujo(id, motivoCierreError, true)
		return
	}
	select {
	case <-f.done:
		return
	default:
	}
	select {
	case <-c.done:
		return
	default:
	}
	if _, err := f.conn.Write(datos); err != nil {
		c.cerrarFlujo(id, motivoConexionRechazada, true)
	}
}

// leerRed bloquea la lectura del socket cuando la cola esta llena: es la
// contrapresion que mantiene las cotas de memoria sin descartar datos.
func (c *conexion) leerRed(f *flujo) {
	tam := tamMaxDatos
	if f.tipo == tipoFlujoUDP {
		tam = tamMaxDatagrama
	}
	buf := make([]byte, tam)
	for {
		n, err := f.conn.Read(buf)
		if n > 0 && !c.colaDatos(f, buf[:n]) {
			return
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("wispd: flujo %d: %v", f.id, err)
			}
			c.cerrarFlujo(f.id, motivoCierreVoluntario, true)
			return
		}
	}
}

func (c *conexion) colaDatos(f *flujo, datos []byte) bool {
	for len(datos) > 0 {
		n := min(len(datos), tamMaxDatos)
		if !c.tomar(f.done) {
			return false
		}
		pkt := codificarPaquete(tipoData, f.id, datos[:n])
		select {
		case f.salida <- pkt:
		case <-f.done:
			c.liberar()
			return false
		case <-c.done:
			c.liberar()
			return false
		}
		datos = datos[n:]
	}
	return true
}

func (c *conexion) escribirRed(f *flujo) {
	for {
		select {
		case pkt := <-f.salida:
			c.escribe.Lock()
			err := c.ws.WriteMessage(websocket.BinaryMessage, pkt)
			c.escribe.Unlock()
			c.liberar()
			if err != nil {
				c.terminar()
				return
			}
		case <-f.done:
			c.drenar(f)
			return
		case <-c.done:
			c.drenar(f)
			return
		}
	}
}

func (c *conexion) cerrarFlujo(id uint32, motivo byte, avisar bool) {
	c.mu.Lock()
	f, existe := c.flujos[id]
	if existe {
		delete(c.flujos, id)
	}
	c.mu.Unlock()
	if !existe {
		return
	}
	f.cerrarRed()
	c.drenar(f)
	if avisar {
		_ = c.escribirPaquete(tipoClose, id, []byte{motivo})
	}
}

func (c *conexion) terminar() {
	c.unaVez.Do(func() {
		close(c.done)
		_ = c.ws.Close()
		c.mu.Lock()
		flujos := make([]*flujo, 0, len(c.flujos))
		for _, f := range c.flujos {
			flujos = append(flujos, f)
		}
		c.mu.Unlock()
		for _, f := range flujos {
			f.cerrarRed()
			c.drenar(f)
		}
	})
}

func (c *conexion) tomar(done <-chan struct{}) bool {
	select {
	case c.srv.tokens <- struct{}{}:
		return true
	case <-done:
	case <-c.done:
	}
	return false
}

func (c *conexion) liberar() {
	select {
	case <-c.srv.tokens:
	default:
	}
}

func (c *conexion) drenar(f *flujo) {
	for {
		select {
		case <-f.salida:
			c.liberar()
		default:
			return
		}
	}
}
