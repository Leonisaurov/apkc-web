package main

// Cliente del protocolo WISP v1.2, escrito aquí y sin dependencias de terceros.
//
// WISP es un proxy de sockets TCP sobre un único WebSocket binario: cada conexión
// es un flujo multiplexado por un `streamID` de 32 bits. El protocolo lo define
// MercuryWorkshop y su especificación es CC-BY-4.0; esta implementación es propia
// y solo reutiliza el formato:
//
//	WISP v1.2, Copyright (c) ading2210, CC-BY-4.0
//	https://github.com/MercuryWorkshop/wisp-protocol
//
// El paquete es `[tipo u8][streamID u32 LE][carga...]`, todo little-endian y las
// cadenas UTF-8 sin terminador NUL. Los tipos que usa el cliente son:
//
//	0x01 CONNECT  carga `[streamType u8][dstPort u16 LE][hostname UTF-8]`
//	0x02 DATA     bytes crudos del socket
//	0x03 CONTINUE carga `[bufferRemaining u32 LE]`, en paquetes, no en bytes
//	0x04 CLOSE    carga `[reason u8]`
//
// El relay manda un CONTINUE inicial con `streamID=0` que fija el crédito de los
// flujos nuevos; hay que esperarlo antes de mandar CONNECT. El valor de
// `bufferRemaining` cambia de un relay a otro (128 en unos, 2147483647 en otros),
// así que no se asume ninguno.
//
// Un mensaje del WebSocket puede llevar varios paquetes seguidos. DATA no tiene
// longitud en la cabecera: se come el resto del mensaje, de modo que DATA solo
// puede ir al final de un mensaje. El decodificador acepta esa coalescencia y
// rechaza un tipo desconocido, porque sin longitud no se puede seguir leyendo.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

const (
	tipoWISPConnect  byte = 0x01
	tipoWISPData     byte = 0x02
	tipoWISPContinue byte = 0x03
	tipoWISPClose    byte = 0x04
)

const (
	// tipoFlujoTCP es el único tipo de flujo que entiende la v1 del protocolo.
	tipoFlujoTCP byte = 0x01

	// Motivos de cierre genéricos del protocolo y el que usa el cliente. Los
	// motivos 0x41-0x49 los reserva el servidor; se aceptan sin interpretarlos.
	motivoCierreGenerico   byte = 0x01
	motivoCierreVoluntario byte = 0x02
	motivoCierreCliente    byte = 0x81
)

// cabeceraWISP son el tipo y el streamID que preceden a cualquier carga.
const cabeceraWISP = 5

// tamMaxDatosWISP parte un Write grande en varios paquetes DATA. El límite es
// conservador: el crédito se cuenta en paquetes y un mensaje demasiado grande
// puede incomodar a relays con buffers pequeños.
const tamMaxDatosWISP = 16 << 10

// plazoInicialWISP acota la espera del CONTINUE inicial para que un relay mudo
// no deje colgada una descarga.
const plazoInicialWISP = 10 * time.Second

// TransporteWISP es el WebSocket visto desde el cliente: mensajes binarios
// completos, en los dos sentidos, y un cierre. En `js/wasm` lo implementa el
// WebSocket del navegador (`wispws_js.go`); en los tests nativos, un par en
// memoria (`wispmemoria.go`).
type TransporteWISP interface {
	Enviar([]byte) error
	Recibir() ([]byte, error)
	Cerrar() error
}

// paqueteWISP es un paquete ya separado del mensaje que lo traía.
type paqueteWISP struct {
	tipo  byte
	flujo uint32
	carga []byte
}

// codificarPaquete arma un paquete completo con su cabecera.
func codificarPaquete(tipo byte, flujo uint32, carga []byte) []byte {
	b := make([]byte, cabeceraWISP+len(carga))
	b[0] = tipo
	binary.LittleEndian.PutUint32(b[1:cabeceraWISP], flujo)
	copy(b[cabeceraWISP:], carga)
	return b
}

// paqueteConnect arma un CONNECT TCP para un hostname y un puerto.
func paqueteConnect(flujo uint32, host string, puerto uint16) []byte {
	carga := make([]byte, 3+len(host))
	carga[0] = tipoFlujoTCP
	binary.LittleEndian.PutUint16(carga[1:3], puerto)
	copy(carga[3:], host)
	return codificarPaquete(tipoWISPConnect, flujo, carga)
}

// paqueteContinue arma un CONTINUE con el crédito en paquetes.
func paqueteContinue(flujo, restantes uint32) []byte {
	carga := make([]byte, 4)
	binary.LittleEndian.PutUint32(carga, restantes)
	return codificarPaquete(tipoWISPContinue, flujo, carga)
}

// paqueteData arma un DATA con bytes crudos.
func paqueteData(flujo uint32, datos []byte) []byte {
	return codificarPaquete(tipoWISPData, flujo, datos)
}

// paqueteClose arma un CLOSE con su motivo.
func paqueteClose(flujo uint32, motivo byte) []byte {
	return codificarPaquete(tipoWISPClose, flujo, []byte{motivo})
}

// decodificarPaquetes separa todos los paquetes de un mensaje del WebSocket. DATA
// se lleva el resto del mensaje, así que si va acompañada tiene que ser la última.
// Un mensaje truncado o con un tipo desconocido es un error de protocolo.
func decodificarPaquetes(mensaje []byte) ([]paqueteWISP, error) {
	paquetes := []paqueteWISP{}
	for len(mensaje) > 0 {
		if len(mensaje) < cabeceraWISP {
			return nil, fmt.Errorf("wisp: mensaje truncado: %d bytes sin cabecera", len(mensaje))
		}
		tipo := mensaje[0]
		flujo := binary.LittleEndian.Uint32(mensaje[1:cabeceraWISP])
		carga := mensaje[cabeceraWISP:]
		switch tipo {
		case tipoWISPData:
			paquetes = append(paquetes, paqueteWISP{tipo: tipo, flujo: flujo, carga: carga})
			mensaje = nil
		case tipoWISPConnect:
			if len(carga) < 3 {
				return nil, fmt.Errorf("wisp: CONNECT truncado: %d bytes", len(carga))
			}
			paquetes = append(paquetes, paqueteWISP{tipo: tipo, flujo: flujo, carga: carga})
			mensaje = nil
		case tipoWISPContinue:
			if len(carga) < 4 {
				return nil, fmt.Errorf("wisp: CONTINUE truncado: %d bytes", len(carga))
			}
			paquetes = append(paquetes, paqueteWISP{tipo: tipo, flujo: flujo, carga: carga[:4]})
			mensaje = carga[4:]
		case tipoWISPClose:
			if len(carga) < 1 {
				return nil, fmt.Errorf("wisp: CLOSE sin motivo")
			}
			paquetes = append(paquetes, paqueteWISP{tipo: tipo, flujo: flujo, carga: carga[:1]})
			mensaje = carga[1:]
		default:
			return nil, fmt.Errorf("wisp: tipo de paquete 0x%02x desconocido", tipo)
		}
	}
	return paquetes, nil
}

// errorCierreWISP describe un cierre ordenado por el relay.
type errorCierreWISP struct {
	flujo  uint32
	motivo byte
}

func (e *errorCierreWISP) Error() string {
	return fmt.Sprintf("wisp: el relay cerró el flujo %d con el motivo 0x%02x", e.flujo, e.motivo)
}

// direccionWISP es la dirección que devuelve un flujo WISP. No hay IP local ni
// remota reales: la conexión sale por el relay.
type direccionWISP string

func (d direccionWISP) Network() string { return "wisp" }
func (d direccionWISP) String() string  { return string(d) }

// ClienteWISP multiplexa flujos sobre un TransporteWISP. Se puede compartir entre
// gorutinas: el envío se serializa y el reparto de paquetes ocurre en un único
// bucle de lectura.
type ClienteWISP struct {
	transporte TransporteWISP

	envioMu sync.Mutex

	mu           sync.Mutex
	flujos       map[uint32]*flujoWISP
	siguienteID  uint32
	buffer       uint32
	inicialListo bool
	err          error

	inicial chan struct{}
	cerrado chan struct{}
	unaVez  sync.Once
}

// NuevoClienteWISP arranca el bucle que reparte los paquetes del transporte.
func NuevoClienteWISP(transporte TransporteWISP) *ClienteWISP {
	c := &ClienteWISP{
		transporte:  transporte,
		flujos:      map[uint32]*flujoWISP{},
		siguienteID: 1,
		inicial:     make(chan struct{}),
		cerrado:     make(chan struct{}),
	}
	go c.bucleLectura()
	return c
}

// Cerrar cierra el transporte y todos los flujos.
func (c *ClienteWISP) Cerrar() error {
	c.fallar(net.ErrClosed)
	return nil
}

// Err devuelve el motivo por el que se cerró el cliente, si se cerró.
func (c *ClienteWISP) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *ClienteWISP) bucleLectura() {
	for {
		mensaje, err := c.transporte.Recibir()
		if err != nil {
			c.fallar(err)
			return
		}
		paquetes, err := decodificarPaquetes(mensaje)
		if err != nil {
			c.fallar(err)
			return
		}
		for _, p := range paquetes {
			if !c.despachar(p) {
				return
			}
		}
	}
}

// despachar aplica un paquete. Devuelve false cuando el cliente ya se cerró.
func (c *ClienteWISP) despachar(p paqueteWISP) bool {
	switch p.tipo {
	case tipoWISPContinue:
		restantes := binary.LittleEndian.Uint32(p.carga)
		if p.flujo == 0 {
			c.mu.Lock()
			if !c.inicialListo {
				c.buffer = restantes
				c.inicialListo = true
				close(c.inicial)
			}
			c.mu.Unlock()
			return true
		}
		if f := c.flujo(p.flujo); f != nil {
			f.fijarCredito(int64(restantes))
		}
	case tipoWISPData:
		if f := c.flujo(p.flujo); f != nil {
			f.entregar(p.carga)
		}
	case tipoWISPClose:
		if f := c.flujo(p.flujo); f != nil {
			var err error
			if motivo := p.carga[0]; motivo == motivoCierreVoluntario {
				err = io.EOF
			} else {
				err = &errorCierreWISP{flujo: p.flujo, motivo: motivo}
			}
			f.cerrarCon(err)
		}
	case tipoWISPConnect:
		// El servidor no manda CONNECT; se ignora para no tumbar un flujo que
		// quizá solo está probando la implementación.
	}
	return true
}

// AbrirTCP abre un flujo WISP hacia `host:puerto`. Espera el CONTINUE inicial,
// elige el streamID (el cliente es quien lo elige) y manda CONNECT. El flujo
// devuelto implementa net.Conn.
func (c *ClienteWISP) AbrirTCP(host string, puerto uint16) (net.Conn, error) {
	if host == "" {
		return nil, errors.New("wisp: hostname vacío")
	}
	if puerto == 0 {
		return nil, errors.New("wisp: puerto 0 no es válido")
	}
	for i := 0; i < len(host); i++ {
		if host[i] == 0 {
			return nil, errors.New("wisp: el hostname lleva un NUL")
		}
	}
	select {
	case <-c.inicial:
	case <-c.cerrado:
		return nil, c.Err()
	case <-time.After(plazoInicialWISP):
		return nil, errors.New("wisp: el relay no mandó el CONTINUE inicial")
	}

	c.mu.Lock()
	if c.err != nil {
		err := c.err
		c.mu.Unlock()
		return nil, err
	}
	id := c.siguienteID
	c.siguienteID++
	f := nuevoFlujoWISP(c, id, host, puerto, c.buffer)
	c.flujos[id] = f
	c.mu.Unlock()

	if err := c.enviar(paqueteConnect(id, host, puerto)); err != nil {
		f.cerrarCon(err)
		return nil, err
	}
	return f, nil
}

// enviar serializa los mensajes del WebSocket: un WS no admite dos escrituras a
// la vez desde la misma conexión.
func (c *ClienteWISP) enviar(b []byte) error {
	select {
	case <-c.cerrado:
		return c.Err()
	default:
	}
	c.envioMu.Lock()
	defer c.envioMu.Unlock()
	select {
	case <-c.cerrado:
		return c.Err()
	default:
	}
	return c.transporte.Enviar(b)
}

func (c *ClienteWISP) flujo(id uint32) *flujoWISP {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.flujos[id]
}

func (c *ClienteWISP) olvidar(id uint32) {
	c.mu.Lock()
	delete(c.flujos, id)
	c.mu.Unlock()
}

// fallar cierra el cliente una sola vez y propaga el motivo a cada flujo.
func (c *ClienteWISP) fallar(err error) {
	c.unaVez.Do(func() {
		c.mu.Lock()
		c.err = err
		flujos := make([]*flujoWISP, 0, len(c.flujos))
		for _, f := range c.flujos {
			flujos = append(flujos, f)
		}
		c.flujos = map[uint32]*flujoWISP{}
		c.mu.Unlock()

		close(c.cerrado)
		_ = c.transporte.Cerrar()
		for _, f := range flujos {
			f.cerrarCon(err)
		}
	})
}

// colaBytes es una cola sin límite con aviso de un solo hueco. El bucle de
// lectura del cliente nunca se bloquea al entregar datos a un flujo, aunque su
// lector vaya lento: si se bloqueara, tampoco podría repartir los CONTINUE de los
// otros flujos y todo el multiplexado se detendría.
type colaBytes struct {
	mu        sync.Mutex
	trozos    [][]byte
	errCierre error
	cerrada   chan struct{}
	aviso     chan struct{}
}

func nuevaColaBytes() *colaBytes {
	return &colaBytes{
		cerrada: make(chan struct{}),
		aviso:   make(chan struct{}, 1),
	}
}

func (q *colaBytes) poner(b []byte) {
	if len(b) == 0 {
		return
	}
	copia := make([]byte, len(b))
	copy(copia, b)
	q.mu.Lock()
	select {
	case <-q.cerrada:
		q.mu.Unlock()
		return
	default:
	}
	q.trozos = append(q.trozos, copia)
	q.mu.Unlock()
	q.despertar()
}

func (q *colaBytes) despertar() {
	select {
	case q.aviso <- struct{}{}:
	default:
	}
}

// tomar espera un trozo hasta el plazo indicado. Un plazo cero significa esperar
// sin límite.
func (q *colaBytes) tomar(hasta time.Time) ([]byte, error) {
	for {
		q.mu.Lock()
		if len(q.trozos) > 0 {
			trozo := q.trozos[0]
			q.trozos = q.trozos[1:]
			q.mu.Unlock()
			return trozo, nil
		}
		q.mu.Unlock()

		// Si la cola se cierra entre este punto y el select, el canal cerrado gana;
		// por eso el caso de cierre lee el motivo con `errorCierre` en vez de
		// confiar en una instantánea anterior.
		var temporizador *time.Timer
		var vencimiento <-chan time.Time
		if !hasta.IsZero() {
			restante := time.Until(hasta)
			if restante <= 0 {
				return nil, os.ErrDeadlineExceeded
			}
			temporizador = time.NewTimer(restante)
			vencimiento = temporizador.C
		}
		select {
		case <-q.aviso:
			if temporizador != nil {
				temporizador.Stop()
			}
		case <-q.cerrada:
			if temporizador != nil {
				temporizador.Stop()
			}
			return nil, q.errorCierre()
		case <-vencimiento:
			return nil, os.ErrDeadlineExceeded
		}
	}
}

func (q *colaBytes) errorCierre() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.errCierre != nil {
		return q.errCierre
	}
	return net.ErrClosed
}

func (q *colaBytes) cerrar(err error) {
	q.mu.Lock()
	select {
	case <-q.cerrada:
		q.mu.Unlock()
		return
	default:
	}
	q.errCierre = err
	close(q.cerrada)
	q.mu.Unlock()
	q.despertar()
}

// flujoWISP es un stream concreto y cumple net.Conn.
type flujoWISP struct {
	cliente *ClienteWISP
	id      uint32
	host    string
	puerto  uint16

	cola      *colaBytes
	lecturaMu sync.Mutex
	resto     []byte

	mu             sync.Mutex
	credito        int64
	avisoCredito   chan struct{}
	cerradoFlag    bool
	errCierre      error
	plazoLectura   time.Time
	plazoEscritura time.Time

	cerrada chan struct{}
}

func nuevoFlujoWISP(c *ClienteWISP, id uint32, host string, puerto uint16, credito uint32) *flujoWISP {
	return &flujoWISP{
		cliente:      c,
		id:           id,
		host:         host,
		puerto:       puerto,
		cola:         nuevaColaBytes(),
		credito:      int64(credito),
		avisoCredito: make(chan struct{}, 1),
		cerrada:      make(chan struct{}),
	}
}

// Read entrega los bytes que manda el relay por este flujo.
func (f *flujoWISP) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	f.lecturaMu.Lock()
	defer f.lecturaMu.Unlock()

	if len(f.resto) > 0 {
		n := copy(b, f.resto)
		f.resto = f.resto[n:]
		return n, nil
	}

	f.mu.Lock()
	plazo := f.plazoLectura
	f.mu.Unlock()

	trozo, err := f.cola.tomar(plazo)
	if err != nil {
		return 0, err
	}
	n := copy(b, trozo)
	if n < len(trozo) {
		f.resto = trozo[n:]
	}
	return n, nil
}

// Write manda los bytes como paquetes DATA, respetando el crédito que anuncia el
// relay. Cada paquete consume una unidad de `bufferRemaining`.
func (f *flujoWISP) Write(b []byte) (int, error) {
	if err := f.comprobarAbierta(); err != nil {
		return 0, err
	}
	total := 0
	for len(b) > 0 {
		n := len(b)
		if n > tamMaxDatosWISP {
			n = tamMaxDatosWISP
		}
		if err := f.tomarCredito(); err != nil {
			return total, err
		}
		if err := f.cliente.enviar(paqueteData(f.id, b[:n])); err != nil {
			return total, err
		}
		total += n
		b = b[n:]
	}
	return total, nil
}

// tomarCredito espera a que el relay anuncie capacidad y consume una unidad.
func (f *flujoWISP) tomarCredito() error {
	for {
		f.mu.Lock()
		if f.cerradoFlag {
			err := f.errCierre
			f.mu.Unlock()
			return err
		}
		if f.credito > 0 {
			f.credito--
			f.mu.Unlock()
			return nil
		}
		plazo := f.plazoEscritura
		f.mu.Unlock()

		var temporizador *time.Timer
		var vencimiento <-chan time.Time
		if !plazo.IsZero() {
			restante := time.Until(plazo)
			if restante <= 0 {
				return os.ErrDeadlineExceeded
			}
			temporizador = time.NewTimer(restante)
			vencimiento = temporizador.C
		}
		select {
		case <-f.avisoCredito:
			if temporizador != nil {
				temporizador.Stop()
			}
		case <-f.cerrada:
			if temporizador != nil {
				temporizador.Stop()
			}
			return f.errorCierre()
		case <-vencimiento:
			return os.ErrDeadlineExceeded
		}
	}
}

// fijarCredito aplica el `bufferRemaining` absoluto que anuncia un CONTINUE.
func (f *flujoWISP) fijarCredito(restantes int64) {
	f.mu.Lock()
	if !f.cerradoFlag {
		f.credito = restantes
	}
	f.mu.Unlock()
	select {
	case f.avisoCredito <- struct{}{}:
	default:
	}
}

func (f *flujoWISP) entregar(b []byte) {
	f.cola.poner(b)
}

func (f *flujoWISP) cerrarCon(err error) {
	f.mu.Lock()
	if f.cerradoFlag {
		f.mu.Unlock()
		return
	}
	f.cerradoFlag = true
	f.errCierre = err
	close(f.cerrada)
	f.mu.Unlock()

	f.cola.cerrar(err)
	f.cliente.olvidar(f.id)
	select {
	case f.avisoCredito <- struct{}{}:
	default:
	}
}

func (f *flujoWISP) errorCierre() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errCierre != nil {
		return f.errCierre
	}
	return net.ErrClosed
}

// comprobarAbierta devuelve el motivo del cierre sin esperar crédito. Así un
// Write vacío sobre un flujo cerrado también falla, como en un net.Conn normal.
func (f *flujoWISP) comprobarAbierta() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cerradoFlag {
		if f.errCierre != nil {
			return f.errCierre
		}
		return net.ErrClosed
	}
	return nil
}

// Close manda el CLOSE del cliente (0x81) y da por terminado el flujo.
func (f *flujoWISP) Close() error {
	f.mu.Lock()
	yaCerrado := f.cerradoFlag
	f.mu.Unlock()
	if yaCerrado {
		return nil
	}
	_ = f.cliente.enviar(paqueteClose(f.id, motivoCierreCliente))
	f.cerrarCon(net.ErrClosed)
	return nil
}

func (f *flujoWISP) LocalAddr() net.Addr {
	return direccionWISP("wisp:cliente")
}

func (f *flujoWISP) RemoteAddr() net.Addr {
	return direccionWISP(net.JoinHostPort(f.host, fmt.Sprintf("%d", f.puerto)))
}

func (f *flujoWISP) SetDeadline(t time.Time) error {
	f.mu.Lock()
	f.plazoLectura = t
	f.plazoEscritura = t
	f.mu.Unlock()
	return nil
}

func (f *flujoWISP) SetReadDeadline(t time.Time) error {
	f.mu.Lock()
	f.plazoLectura = t
	f.mu.Unlock()
	return nil
}

func (f *flujoWISP) SetWriteDeadline(t time.Time) error {
	f.mu.Lock()
	f.plazoEscritura = t
	f.mu.Unlock()
	return nil
}
