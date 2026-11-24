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
	"crypto/rand"
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

	// Motivos de cierre genéricos del protocolo. Los motivos 0x41-0x49 los
	// reserva el servidor; se aceptan sin interpretarlos.
	motivoCierreGenerico   byte = 0x01
	motivoCierreVoluntario byte = 0x02
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

// Topes de memoria del cliente. Los flujos acumulan en una cola los bytes que
// llegan del relay hasta que el lector los pide; sin tope, un relay hostil (o
// un lector atascado) puede llenar el heap de la pestaña, que en wasm es el del
// proceso entero. Las cifras salen de lo medido: el artefacto mayor que se
// descarga es `android.jar`, de ~26 MB, y el lector de TLS lo consume en
// trozos, así que la cola solo guarda lo que aún no se ha leído (unas decenas
// de KB en régimen normal).
const (
	// topeBytesFlujoWISP: 16 MiB por flujo (1024 paquetes de 16 KiB). Da más de
	// mil veces la ventana que ocupa un lector normal, y deja al flujo por
	// debajo del jar de 26 MB: un flujo atascado no puede esconder un artefacto
	// entero en memoria.
	topeBytesFlujoWISP = 16 << 20

	// topeBytesGlobalWISP: 64 MiB entre todos los flujos (cuatro flujos a tope).
	// El heap inicial de un wasm de Go son 256 MiB y el compilador necesita el
	// resto; con este tope, la cola no puede ser la causa de un OOM de la
	// pestaña. Y como el transporte rechaza una trama mayor que
	// `topeMensajeWISP` antes de convertirla, una sola trama del relay tampoco
	// puede saltarse estos números.
	topeBytesGlobalWISP = 64 << 20

	// creditoMaximoWISP acota el crédito de envío que se acepta de un CONTINUE,
	// que viene en paquetes. Un relay puede anunciar 0xFFFFFFFF (4.294.967.295
	// paquetes), y con el tamaño máximo de DATA serían 64 GiB de credibilidad
	// para inundar el socket: el tope son 1024 paquetes, los mismos 16 MiB que
	// la cola de un flujo.
	creditoMaximoWISP = 1024
)

// ErrColaDesbordada marca el cierre de un flujo porque su cola superó el tope de
// bytes. El flujo se cierra con un CLOSE y el lector recibe este error en vez de
// esperar a que la memoria crezca.
var ErrColaDesbordada = errors.New("wisp: la cola del flujo superó su tope de bytes sin lector")

// topeMensajeWISP es el mayor mensaje del WebSocket que puede ser legítimo: un
// DATA del tamaño máximo de una cola, su cabecera y un margen para los
// CONTINUE/CLOSE que el relay pueda coalescer delante. El transporte del
// navegador lo comprueba sobre `byteLength` antes de convertir el ArrayBuffer,
// así que una sola trama de N bytes no cuesta N bytes de heap para rechazarla.
// DATA no lleva longitud y ocupa el mensaje entero, de modo que una sola trama
// basta para intentarlo.
const topeMensajeWISP = topeBytesFlujoWISP + cabeceraWISP + 64

// plazoDrenajeWISP es el máximo que se espera a que el WebSocket vacíe
// `bufferedAmount` cuando el flujo no ha fijado un plazo de escritura. Con
// plazo, manda el del flujo.
const plazoDrenajeWISP = 10 * time.Second

// errorFlujoWISP envuelve un fallo que afecta a un solo flujo, como una trama
// demasiado grande descartada en el transporte: el bucle de lectura cierra ese
// flujo y sigue con los demás en vez de tumbar la conexión.
type errorFlujoWISP struct {
	flujo uint32
	err   error
}

func (e *errorFlujoWISP) Error() string { return e.err.Error() }
func (e *errorFlujoWISP) Unwrap() error { return e.err }

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

// comprobarMensajeWISP decide si un mensaje del WebSocket puede convertirse a
// []byte. Se llama con el tamaño anunciado por el navegador (`byteLength`), antes
// de reservar nada: rechazar N bytes no puede costar N bytes.
func comprobarMensajeWISP(largo int) error {
	if largo > topeMensajeWISP {
		return fmt.Errorf("%w: trama de %d bytes con tope de mensaje %d",
			ErrColaDesbordada, largo, topeMensajeWISP)
	}
	return nil
}

// flujoDeTramaGrandeWISP busca el streamID del DATA de una trama que se descarta
// sin convertirla. Solo mira la cabecera y los CONTINUE/CLOSE que la precedan; si
// el primer paquete no permite seguir, devuelve su flujo y el llamador decide.
func flujoDeTramaGrandeWISP(prefix []byte) (uint32, bool) {
	for len(prefix) >= cabeceraWISP {
		tipo := prefix[0]
		flujo := binary.LittleEndian.Uint32(prefix[1:cabeceraWISP])
		switch tipo {
		case tipoWISPData:
			return flujo, true
		case tipoWISPContinue:
			if len(prefix) < cabeceraWISP+4 {
				return flujo, true
			}
			prefix = prefix[cabeceraWISP+4:]
		case tipoWISPClose:
			if len(prefix) < cabeceraWISP+1 {
				return flujo, true
			}
			prefix = prefix[cabeceraWISP+1:]
		default:
			return flujo, true
		}
	}
	return 0, false
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

	// topeFlujo es la cota de bytes de la cola de cada flujo y topeGlobal la del
	// conjunto. Se copian del valor por defecto al construir el cliente y las
	// pruebas pueden reducirlos antes de abrir el primer flujo.
	topeFlujo  int
	topeGlobal *topeBytes

	// avisos son los mensajes de control (CLOSE de desborde) que manda una
	// gorutina aparte para que el bucle de lectura no espere al transporte.
	avisos chan []byte

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
		topeFlujo:   topeBytesFlujoWISP,
		topeGlobal:  nuevoTopeBytes(topeBytesGlobalWISP),
		avisos:      make(chan []byte, 8),
		inicial:     make(chan struct{}),
		cerrado:     make(chan struct{}),
	}
	go c.bucleLectura()
	go c.bombeoAvisos()
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
			// Un fallo de flujo (una trama descartada por tamaño) cierra solo ese
			// flujo; cualquier otro error tumba el cliente.
			var fallo *errorFlujoWISP
			if errors.As(err, &fallo) {
				if f := c.flujo(fallo.flujo); f != nil {
					c.avisarSinBloquear(paqueteClose(fallo.flujo, motivoCierreGenerico))
					f.cerrarCon(fallo.err)
				}
				continue
			}
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
			if err := f.entregar(p.carga); err != nil {
				// La cola no crece más: se avisa al relay con un CLOSE y se cierra
				// el flujo con el motivo. El aviso va por una cola aparte para que
				// este bucle nunca espere al transporte (el drenaje del WebSocket
				// puede tardar segundos y pararía todos los flujos).
				c.avisarSinBloquear(paqueteClose(p.flujo, motivoCierreGenerico))
				f.cerrarCon(err)
			}
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
//
// Esta es la primitiva del protocolo y no aplica la allowlist: los tests abren
// destinos arbitrarios contra un relay en memoria. El camino seguro es
// `AbrirTCPAutorizado` (o `AbrirTLSWISP`, que lo usa), que comprueba el destino
// antes de mandar el CONNECT.
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
	id := c.elegirID()
	f := nuevoFlujoWISP(c, id, host, puerto, c.buffer)
	c.flujos[id] = f
	c.mu.Unlock()

	if err := c.enviar(paqueteConnect(id, host, puerto)); err != nil {
		f.cerrarCon(err)
		return nil, err
	}
	return f, nil
}

// AbrirTCPAutorizado es la entrada del camino seguro: comprueba la pareja
// host/puerto contra la allowlist de la configuración antes de mandar el CONNECT.
// Un destino que no esté autorizado se rechaza sin tocar el relay.
func (c *ClienteWISP) AbrirTCPAutorizado(cfg ConfigWISP, host string, puerto uint16) (net.Conn, error) {
	if err := cfg.ValidarDestino(host, puerto); err != nil {
		return nil, err
	}
	return c.AbrirTCP(host, puerto)
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

// transporteConPlazoWISP lo implementa el transporte que sabe esperar a que su
// búfer drene (el WebSocket del navegador). El de memoria no lo implementa y el
// plazo se ignora allí, donde `Enviar` no acumula.
type transporteConPlazoWISP interface {
	EnviarConPlazo(b []byte, hasta time.Time) error
}

// enviarConPlazo es `enviar` con el plazo de escritura del flujo. El envío sigue
// serializado: el drenaje del WebSocket no admite dos escrituras a la vez.
func (c *ClienteWISP) enviarConPlazo(b []byte, hasta time.Time) error {
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
	if tp, ok := c.transporte.(transporteConPlazoWISP); ok {
		return tp.EnviarConPlazo(b, hasta)
	}
	return c.transporte.Enviar(b)
}

// avisarSinBloquear encola un mensaje de control (el CLOSE de un flujo que se
// desborda) sin esperar al transporte. La cola tiene tope: si está llena el
// aviso se descarta, porque el flujo ya está cerrado en este lado y los DATA que
// lleguen para él se desechan. El bucle de lectura nunca se queda esperando al
// socket por un CLOSE.
func (c *ClienteWISP) avisarSinBloquear(b []byte) {
	select {
	case c.avisos <- b:
	default:
	}
}

// bombeoAvisos manda los avisos de control en su propia gorutina para que el
// reparto de paquetes siga. Termina cuando se cierra el cliente.
func (c *ClienteWISP) bombeoAvisos() {
	for {
		select {
		case <-c.cerrado:
			return
		case b := <-c.avisos:
			_ = c.enviar(b)
		}
	}
}

// esperarDrenajeWISP espera a que `pendiente` baje del umbral. `hasta` es el
// plazo del Write y manda sobre `plazoDrenajeWISP`: sin esto, un Write con
// deadline corto podía esperar 10 s por cada paquete. `reloj` y `dormir` se
// inyectan para probarlo sin navegador.
func esperarDrenajeWISP(pendiente func() (int, bool), umbral int, hasta time.Time, reloj func() time.Time, dormir func(time.Duration)) error {
	limite := reloj().Add(plazoDrenajeWISP)
	if !hasta.IsZero() && hasta.Before(limite) {
		limite = hasta
	}
	for {
		enCola, ok := pendiente()
		if !ok || enCola <= umbral {
			return nil
		}
		if !reloj().Before(limite) {
			return fmt.Errorf("%w: el WebSocket lleva %d bytes sin drenar (umbral %d)",
				os.ErrDeadlineExceeded, enCola, umbral)
		}
		dormir(5 * time.Millisecond)
	}
}

// elegirID escoge el streamID de un flujo nuevo. Se llama con `c.mu` tomado.
// Antes era un contador que al desbordar daba 0 (reservado para los CONTINUE de
// control); ahora es aleatorio, con el contador como red de seguridad si
// `crypto/rand` fallara, y nunca pisa un flujo vivo.
func (c *ClienteWISP) elegirID() uint32 {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err == nil {
		if id := binary.LittleEndian.Uint32(buf[:]); id != 0 && c.flujos[id] == nil {
			return id
		}
	}
	for {
		c.siguienteID++
		if c.siguienteID == 0 {
			c.siguienteID = 1
		}
		if c.flujos[c.siguienteID] == nil {
			return c.siguienteID
		}
	}
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

// topeBytes es un contador de bytes con límite. Se comparte entre las colas para
// acotar el total que el cliente retiene en memoria.
type topeBytes struct {
	mu     sync.Mutex
	usado  int
	limite int
}

func nuevoTopeBytes(limite int) *topeBytes {
	return &topeBytes{limite: limite}
}

// reservar suma si el bloque cabe; si no cabe no toca el contador.
func (t *topeBytes) reservar(n int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.usado+n > t.limite {
		return false
	}
	t.usado += n
	return true
}

func (t *topeBytes) liberar(n int) {
	t.mu.Lock()
	t.usado -= n
	if t.usado < 0 {
		t.usado = 0
	}
	t.mu.Unlock()
}

func (t *topeBytes) enUso() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.usado
}

func (t *topeBytes) capacidad() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.limite
}

// colaBytes es una cola con cota de bytes y aviso de un solo hueco. El bucle de
// lectura del cliente nunca se bloquea al entregar datos a un flujo, aunque su
// lector vaya lento: si se bloqueara, tampoco podría repartir los CONTINUE de los
// otros flujos y todo el multiplexado se detendría. A cambio, la cola está
// acotada por flujo y en conjunto, y cuando el tope se agota el flujo se cierra
// con un CLOSE en vez de crecer.
type colaBytes struct {
	mu     sync.Mutex
	trozos [][]byte
	usado  int

	topeFlujo  int
	topeGlobal *topeBytes

	errCierre error
	cerrada   chan struct{}
	aviso     chan struct{}
}

func nuevaColaBytes(topeFlujo int, topeGlobal *topeBytes) *colaBytes {
	return &colaBytes{
		topeFlujo:  topeFlujo,
		topeGlobal: topeGlobal,
		cerrada:    make(chan struct{}),
		aviso:      make(chan struct{}, 1),
	}
}

// poner encola una copia del bloque. Comprueba el tamaño antes de copiar, de
// modo que rechazar N bytes no cuesta N bytes de heap. Devuelve
// ErrColaDesbordada sin crecer la cola si no cabe; sobre una cola ya cerrada
// descarta el bloque en silencio, como hacía la versión sin tope.
func (q *colaBytes) poner(b []byte) error {
	n := len(b)
	if n == 0 {
		return nil
	}

	q.mu.Lock()
	select {
	case <-q.cerrada:
		q.mu.Unlock()
		return nil
	default:
	}
	if q.usado+n > q.topeFlujo {
		desborde := q.usado + n
		enCola := q.usado
		tope := q.topeFlujo
		q.mu.Unlock()
		return fmt.Errorf("%w: %d bytes pedidos con %d ya en cola y %d de tope",
			ErrColaDesbordada, desborde, enCola, tope)
	}
	if q.topeGlobal != nil && !q.topeGlobal.reservar(n) {
		usado := q.topeGlobal.enUso()
		capacidad := q.topeGlobal.capacidad()
		q.mu.Unlock()
		return fmt.Errorf("%w: %d bytes pedidos con %d en todos los flujos y %d de tope global",
			ErrColaDesbordada, usado+n, usado, capacidad)
	}
	// Solo aquí, con el hueco ya reservado, se copia. La copia va bajo `q.mu`
	// para que nadie vea bytes contados que aún no están en la cola; el máximo
	// que puede copiarse de una vez es el tope del flujo.
	q.usado += n
	copia := make([]byte, n)
	copy(copia, b)
	q.trozos = append(q.trozos, copia)
	q.mu.Unlock()
	q.despertar()
	return nil
}

func (q *colaBytes) despertar() {
	select {
	case q.aviso <- struct{}{}:
	default:
	}
}

// leer copia lo que haya en la cabeza de la cola. Devuelve (0, nil) si está
// vacía y abierta, y el motivo del cierre si está vacía y cerrada. Si `b` es
// menor que el trozo, el resto se queda en la cola sin copiarse: un Read de un
// byte sobre un DATA grande no duplica la memoria, y la contabilidad baja solo
// por lo entregado.
func (q *colaBytes) leer(b []byte) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.trozos) == 0 {
		select {
		case <-q.cerrada:
			return 0, q.errorCierreEnCaja()
		default:
			return 0, nil
		}
	}
	trozo := q.trozos[0]
	n := copy(b, trozo)
	q.usado -= n
	if q.topeGlobal != nil {
		q.topeGlobal.liberar(n)
	}
	if n == len(trozo) {
		// Soltar el búfer del array que se recorta, para que no quede retenido.
		q.trozos[0] = nil
		q.trozos = q.trozos[1:]
	} else {
		q.trozos[0] = trozo[n:]
	}
	return n, nil
}

// errorCierreEnCaja lee el motivo con la cola ya bloqueada.
func (q *colaBytes) errorCierreEnCaja() error {
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
	// Los bytes pendientes siguen vivos en memoria hasta que el lector los
	// consuma o los suelte: siguen contando en el tope global. Devolverlos aquí
	// declararía libre memoria que aún está retenida (sonda del auditor: 278.528
	// B vivos con el global a 0). La liberación real la hacen `leer` y `soltar`.
	q.mu.Unlock()
	q.despertar()
}

// soltar descarta los trozos pendientes y devuelve su hueco al tope global. Se
// usa cuando el flujo se abandona sin drenar (por ejemplo, un Close propio).
func (q *colaBytes) soltar() {
	q.mu.Lock()
	q.trozos = nil
	if q.topeGlobal != nil {
		q.topeGlobal.liberar(q.usado)
	}
	q.usado = 0
	q.mu.Unlock()
}

// flujoWISP es un stream concreto y cumple net.Conn.
type flujoWISP struct {
	cliente *ClienteWISP
	id      uint32
	host    string
	puerto  uint16

	cola      *colaBytes
	lecturaMu sync.Mutex
	plazos    *plazosWISP

	mu           sync.Mutex
	credito      int64
	avisoCredito chan struct{}
	cerradoFlag  bool
	errCierre    error

	cerrada chan struct{}
}

func nuevoFlujoWISP(c *ClienteWISP, id uint32, host string, puerto uint16, credito uint32) *flujoWISP {
	creditoInicial := int64(credito)
	if creditoInicial > creditoMaximoWISP {
		creditoInicial = creditoMaximoWISP
	}
	return &flujoWISP{
		cliente:      c,
		id:           id,
		host:         host,
		puerto:       puerto,
		cola:         nuevaColaBytes(c.topeFlujo, c.topeGlobal),
		credito:      creditoInicial,
		avisoCredito: make(chan struct{}, 1),
		plazos:       nuevoPlazosWISP(),
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
	for {
		n, err := f.cola.leer(b)
		if n > 0 || err != nil {
			return n, err
		}
		if err := f.esperarBloqueo(true, f.cola.aviso, f.cola.cerrada); err != nil {
			return 0, err
		}
	}
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
		hasta, _ := f.plazos.instantanea(false)
		if err := f.cliente.enviarConPlazo(paqueteData(f.id, b[:n]), hasta); err != nil {
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
			if err != nil {
				return err
			}
			return net.ErrClosed
		}
		if f.credito > 0 {
			f.credito--
			f.mu.Unlock()
			return nil
		}
		f.mu.Unlock()

		if err := f.esperarBloqueo(false, f.avisoCredito, f.cerrada); err != nil {
			return err
		}
	}
}

// esperarBloqueo suelta la gorutina hasta que haya algo que revisar: un aviso de
// la cola o del crédito, el cierre o un cambio de plazo. Sin el canal de plazo,
// un SetReadDeadline sobre una lectura ya bloqueada no la despertaría, que es
// justo lo que exige net.Conn. La espera que cubre es la del crédito (y la de
// la cola); el envío ya al transporte no es interrumpible, pero en wasm está
// acotado por el plazo de drenaje de `bufferedAmount`.
func (f *flujoWISP) esperarBloqueo(lectura bool, aviso, cierre <-chan struct{}) error {
	hasta, cambioPlazo := f.plazos.instantanea(lectura)
	if !hasta.IsZero() {
		restante := time.Until(hasta)
		if restante <= 0 {
			return os.ErrDeadlineExceeded
		}
		temporizador := time.NewTimer(restante)
		defer temporizador.Stop()
		select {
		case <-aviso:
			return nil
		case <-cierre:
			return nil
		case <-cambioPlazo:
			return nil
		case <-temporizador.C:
			return os.ErrDeadlineExceeded
		}
	}
	select {
	case <-aviso:
		return nil
	case <-cierre:
		return nil
	case <-cambioPlazo:
		return nil
	}
}

// fijarCredito aplica el `bufferRemaining` absoluto que anuncia un CONTINUE,
// acotado a creditoMaximoWISP: un relay que anuncia 0xFFFFFFFF no puede
// autorizar 64 GiB de envío.
func (f *flujoWISP) fijarCredito(restantes int64) {
	if restantes < 0 {
		restantes = 0
	}
	if restantes > creditoMaximoWISP {
		restantes = creditoMaximoWISP
	}
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

// entregar encola los bytes de un DATA. Si la cola ya no admite el bloque,
// devuelve ErrColaDesbordada y el llamador cierra el flujo con un CLOSE.
func (f *flujoWISP) entregar(b []byte) error {
	return f.cola.poner(b)
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

// Close manda el CLOSE voluntario (0x02) y da por terminado el flujo.
func (f *flujoWISP) Close() error {
	f.mu.Lock()
	yaCerrado := f.cerradoFlag
	f.mu.Unlock()
	if yaCerrado {
		return nil
	}
	_ = f.cliente.enviar(paqueteClose(f.id, motivoCierreVoluntario))
	f.cerrarCon(net.ErrClosed)
	// Un Close propio abandona lo que quedara sin leer: se descarta y se
	// devuelve su hueco al tope global, que si no quedaría retenido por un flujo
	// que ya nadie va a drenar.
	f.cola.soltar()
	return nil
}

func (f *flujoWISP) LocalAddr() net.Addr {
	return direccionWISP("wisp:cliente")
}

func (f *flujoWISP) RemoteAddr() net.Addr {
	return direccionWISP(net.JoinHostPort(f.host, fmt.Sprintf("%d", f.puerto)))
}

func (f *flujoWISP) SetDeadline(t time.Time) error {
	f.plazos.fijarTodo(t)
	return nil
}

func (f *flujoWISP) SetReadDeadline(t time.Time) error {
	f.plazos.fijarLectura(t)
	return nil
}

func (f *flujoWISP) SetWriteDeadline(t time.Time) error {
	f.plazos.fijarEscritura(t)
	return nil
}

// plazosWISP guarda los plazos de lectura y escritura de un flujo. Cada uno
// tiene un canal que se cierra y se reemplaza al fijar el plazo: el cierre
// despierta a quien estuviera esperando con el valor viejo y el nuevo canal deja
// al siguiente esperador con el valor fresco. Es lo que hace que un
// `SetReadDeadline` interrumpa la lectura pendiente, como en un net.Conn.
type plazosWISP struct {
	mu             sync.Mutex
	lectura        time.Time
	escritura      time.Time
	avisoLectura   chan struct{}
	avisoEscritura chan struct{}
}

func nuevoPlazosWISP() *plazosWISP {
	return &plazosWISP{
		avisoLectura:   make(chan struct{}),
		avisoEscritura: make(chan struct{}),
	}
}

// instantanea devuelve el plazo y el canal que avisa de su próximo cambio.
func (p *plazosWISP) instantanea(lectura bool) (time.Time, <-chan struct{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if lectura {
		return p.lectura, p.avisoLectura
	}
	return p.escritura, p.avisoEscritura
}

func (p *plazosWISP) fijarTodo(t time.Time) {
	p.mu.Lock()
	p.lectura = t
	p.escritura = t
	close(p.avisoLectura)
	p.avisoLectura = make(chan struct{})
	close(p.avisoEscritura)
	p.avisoEscritura = make(chan struct{})
	p.mu.Unlock()
}

func (p *plazosWISP) fijarLectura(t time.Time) {
	p.mu.Lock()
	p.lectura = t
	close(p.avisoLectura)
	p.avisoLectura = make(chan struct{})
	p.mu.Unlock()
}

func (p *plazosWISP) fijarEscritura(t time.Time) {
	p.mu.Lock()
	p.escritura = t
	close(p.avisoEscritura)
	p.avisoEscritura = make(chan struct{})
	p.mu.Unlock()
}
