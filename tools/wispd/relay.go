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
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"syscall"
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
	destinos  politicaDestinos
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
		destinos:   politicaProduccion,
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
			codificarPaquete(tipoClose, 0, []byte{motivoLimitado}))
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

	// Credito en los dos sentidos, en paquetes. `creditoEntrada` lo anuncia este
	// relay al cliente; `creditoSalida` lo anuncia el cliente al relay. Hasta que
	// el cliente envie su primer CONTINUE de flujo se mantiene sin tope: los
	// clientes Wisp v1 de este repo no lo envian.
	mu             sync.Mutex
	cond           *sync.Cond
	creditoEntrada uint32 // paquetes que el cliente aún puede enviar
	creditoSalida  int64  // paquetes que el relay aún puede enviar al cliente
	cerrado        bool
}

func nuevoFlujo(id uint32, tipo byte) *flujo {
	f := &flujo{
		id:            id,
		tipo:          tipo,
		salida:        make(chan []byte, cupoPorFlujo),
		done:          make(chan struct{}),
		creditoSalida: math.MaxInt64,
	}
	f.cond = sync.NewCond(&f.mu)
	return f
}

func (f *flujo) cerrarRed() {
	f.unaVez.Do(func() {
		f.mu.Lock()
		f.cerrado = true
		f.mu.Unlock()
		f.cond.Broadcast()
		close(f.done)
		if f.conn != nil {
			_ = f.conn.Close()
		}
	})
}

// recargarSalida aplica el valor absoluto de un CONTINUE del cliente. Un valor
// 0 se descarta: conceder cero dejaría al relay esperando para siempre.
func (f *flujo) recargarSalida(restantes uint32) {
	if restantes == 0 || f.tipo == tipoFlujoUDP {
		return
	}
	f.mu.Lock()
	f.creditoSalida = int64(restantes)
	f.mu.Unlock()
	f.cond.Broadcast()
}

// tomarSalida reserva un paquete del credito que el cliente concedió. Si se
// agota, espera a un CONTINUE nuevo o al cierre del flujo.
func (f *flujo) tomarSalida() bool {
	if f.tipo == tipoFlujoUDP {
		return true
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for f.creditoSalida <= 0 && !f.cerrado {
		f.cond.Wait()
	}
	if f.cerrado || f.creditoSalida <= 0 {
		return false
	}
	f.creditoSalida--
	return true
}

// contabilizarEntrada descuenta un paquete recibido del cliente y avisa cuándo
// conviene renovar el credito. Nunca se anuncia 0: apkc/wisp.go:894-916 bloquea
// el Write del cliente si el credito llega a cero.
func (f *flujo) contabilizarEntrada(ventana uint32) bool {
	if f.tipo == tipoFlujoUDP {
		return false
	}
	f.mu.Lock()
	if f.creditoEntrada > 0 {
		f.creditoEntrada--
	}
	umbral := ventana / 4
	if umbral < 1 {
		umbral = 1
	}
	renovar := f.creditoEntrada <= umbral
	f.mu.Unlock()
	return renovar
}

func (f *flujo) renovarEntrada(ventana uint32) {
	if ventana == 0 {
		return
	}
	f.mu.Lock()
	f.creditoEntrada = ventana
	f.mu.Unlock()
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

// recibirContinuo aplica el credito que el cliente concede al relay para enviar
// DATA en ese flujo. En UDP no hay contabilidad de buffers (spec v1.2).
func (c *conexion) recibirContinuo(id uint32, restantes uint32) {
	if id == 0 || restantes == 0 {
		return
	}
	c.mu.Lock()
	f := c.flujos[id]
	c.mu.Unlock()
	if f == nil || f.tipo == tipoFlujoUDP {
		return
	}
	f.recargarSalida(restantes)
}

func (c *conexion) procesar(p paquete) {
	switch p.tipo {
	case tipoConnect:
		c.abrirFlujo(p.streamID, p.carga)
	case tipoData:
		c.datosAlSocket(p.streamID, p.carga)
	case tipoContinue:
		// El CONTINUE del cliente fija su credito de recepcion. El de streamID=0
		// no llegue: el inicial lo manda este relay.
		c.recibirContinuo(p.streamID, creditoDe(p))
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
		_ = c.escribirPaquete(tipoClose, id, []byte{motivoInfoInvalida})
		return
	}

	// La allowlist de destinos se aplica antes de resolver y antes de dialear.
	ctx, cancel := context.WithTimeout(context.Background(), plazoDial)
	defer cancel()
	dir, motivo := c.srv.resolverDestino(ctx, sol.host, sol.puerto, sol.tipoFlujo)
	if motivo != 0 {
		_ = c.escribirPaquete(tipoClose, id, []byte{motivo})
		return
	}

	c.mu.Lock()
	if len(c.flujos) >= c.srv.maxFlujos {
		c.mu.Unlock()
		_ = c.escribirPaquete(tipoClose, id, []byte{motivoLimitado})
		return
	}
	if _, existe := c.flujos[id]; existe {
		c.mu.Unlock()
		_ = c.escribirPaquete(tipoClose, id, []byte{motivoCierreGenerico})
		return
	}
	f := nuevoFlujo(id, sol.tipoFlujo)
	f.creditoEntrada = c.srv.credito
	c.flujos[id] = f
	c.mu.Unlock()

	red := "tcp"
	if sol.tipoFlujo == tipoFlujoUDP {
		red = "udp"
	}
	conn, errDial := (&net.Dialer{Timeout: plazoDial}).DialContext(ctx, red, dir)
	if errDial != nil {
		c.cerrarFlujo(id, motivoDial(errDial), true)
		return
	}
	f.conn = conn

	go c.leerRed(f)
	go c.escribirRed(f)
}

// motivoDial traduce el fallo del socket a la tabla de motivos de Wisp v1.2.
func motivoDial(err error) byte {
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return motivoHostInalcanzable
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return motivoTiempoAgotado
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return motivoConexionRechazada
	}
	return motivoErrorRed
}

func (c *conexion) datosAlSocket(id uint32, datos []byte) {
	c.mu.Lock()
	f := c.flujos[id]
	c.mu.Unlock()
	if f == nil {
		_ = c.escribirPaquete(tipoClose, id, []byte{motivoCierreGenerico})
		return
	}
	if f.tipo == tipoFlujoUDP && len(datos) > tamMaxDatagrama {
		c.cerrarFlujo(id, motivoInfoInvalida, true)
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
		c.cerrarFlujo(id, motivoErrorRed, true)
		return
	}
	// El cliente solo puede enviar lo que este CONTINUE le concedió. Antes de
	// agotarse se renueva el crédito, para que su Write nunca se bloquee en 0.
	if f.contabilizarEntrada(c.srv.credito) {
		f.renovarEntrada(c.srv.credito)
		credito := make([]byte, 4)
		binary.LittleEndian.PutUint32(credito, c.srv.credito)
		_ = c.escribirPaquete(tipoContinue, id, credito)
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
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
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
		if !f.tomarSalida() {
			c.liberar()
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
