package main

// Pruebas del framing WISP y del net.Conn, sin red: el relay es un par en
// memoria que habla el mismo protocolo que el relay real.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWISPCodificarYDecodificar(t *testing.T) {
	casos := []struct {
		nombre string
		bytes  []byte
		tipo   byte
		flujo  uint32
		carga  []byte
	}{
		{"CONNECT", paqueteConnect(7, "ejemplo.test", 443), tipoWISPConnect, 7, nil},
		{"DATA", paqueteData(9, []byte("hola")), tipoWISPData, 9, []byte("hola")},
		{"CONTINUE", paqueteContinue(3, 2147483647), tipoWISPContinue, 3, []byte{0xff, 0xff, 0xff, 0x7f}},
		{"CLOSE", paqueteClose(4, motivoCierreCliente), tipoWISPClose, 4, []byte{motivoCierreCliente}},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			paquetes, err := decodificarPaquetes(caso.bytes)
			if err != nil {
				t.Fatalf("decodificar: %v", err)
			}
			if len(paquetes) != 1 {
				t.Fatalf("se esperaba 1 paquete, llegaron %d", len(paquetes))
			}
			p := paquetes[0]
			if p.tipo != caso.tipo || p.flujo != caso.flujo {
				t.Fatalf("cabecera = (%#x, %d), se esperaba (%#x, %d)", p.tipo, p.flujo, caso.tipo, caso.flujo)
			}
			if caso.carga != nil && !bytes.Equal(p.carga, caso.carga) {
				t.Fatalf("carga = %x, se esperaba %x", p.carga, caso.carga)
			}
		})
	}

	connect, err := decodificarPaquetes(paqueteConnect(7, "ejemplo.test", 443))
	if err != nil {
		t.Fatalf("decodificar CONNECT: %v", err)
	}
	carga := connect[0].carga
	if carga[0] != tipoFlujoTCP {
		t.Fatalf("streamType = %#x", carga[0])
	}
	if puerto := binary.LittleEndian.Uint16(carga[1:3]); puerto != 443 {
		t.Fatalf("puerto = %d", puerto)
	}
	if host := string(carga[3:]); host != "ejemplo.test" {
		t.Fatalf("host = %q", host)
	}
}

func TestWISPDescodificarCoalescencia(t *testing.T) {
	// Tres paquetes en un solo mensaje y DATA al final, que es la única forma de
	// que DATA no se coma los siguientes.
	mensaje := append(paqueteContinue(0, 128), paqueteContinue(1, 2)...)
	mensaje = append(mensaje, paqueteClose(1, motivoCierreVoluntario)...)
	mensaje = append(mensaje, paqueteData(1, []byte("cola"))...)

	paquetes, err := decodificarPaquetes(mensaje)
	if err != nil {
		t.Fatalf("decodificar mensaje coalescido: %v", err)
	}
	if len(paquetes) != 4 {
		t.Fatalf("se esperaban 4 paquetes, llegaron %d", len(paquetes))
	}
	if paquetes[0].tipo != tipoWISPContinue || paquetes[0].flujo != 0 {
		t.Fatalf("primer paquete = %#x/%d", paquetes[0].tipo, paquetes[0].flujo)
	}
	if n := binary.LittleEndian.Uint32(paquetes[1].carga); n != 2 {
		t.Fatalf("crédito = %d", n)
	}
	if !bytes.Equal(paquetes[3].carga, []byte("cola")) {
		t.Fatalf("DATA = %q", paquetes[3].carga)
	}
}

func TestWISPDescodificarRechazaTruncados(t *testing.T) {
	casos := map[string][]byte{
		"cabecera corta":   {0x03, 0x00},
		"CONTINUE corto":   {0x03, 0, 0, 0, 0, 0x01},
		"CLOSE sin motivo": {0x04, 0, 0, 0, 0},
		"CONNECT corto":    {0x01, 0, 0, 0, 0, 0x01},
		"tipo desconocido": {0x7f, 0, 0, 0, 0},
	}
	for nombre, mensaje := range casos {
		t.Run(nombre, func(t *testing.T) {
			if _, err := decodificarPaquetes(mensaje); err == nil {
				t.Fatalf("se esperaba error de protocolo para %x", mensaje)
			}
		})
	}
}

// flujoRelay es el lado del relay para un stream: un pipe hacia el servidor de
// pruebas (o nada, si el relay hace eco).
type flujoRelay struct {
	id      uint32
	lado    net.Conn
	entrada chan []byte
	cerrado chan struct{}
	unaVez  sync.Once
}

type relayPrueba struct {
	t          *testing.T
	extremo    *extremoMemoria
	eco        bool
	alConectar func(uint32) net.Conn

	envioMu sync.Mutex

	mu        sync.Mutex
	flujos    map[uint32]*flujoRelay
	capturado bytes.Buffer
	datos     int
}

// nuevoRelayPrueba arranca un relay en memoria que manda el CONTINUE inicial y
// atiende los flujos del cliente.
func nuevoRelayPrueba(t *testing.T, buffer uint32, eco bool, alConectar func(uint32) net.Conn) (*relayPrueba, *extremoMemoria) {
	t.Helper()
	extremoCliente, extremoServidor := nuevoParMemoria()
	r := &relayPrueba{
		t:          t,
		extremo:    extremoServidor,
		eco:        eco,
		alConectar: alConectar,
		flujos:     map[uint32]*flujoRelay{},
	}
	t.Cleanup(func() {
		_ = r.extremo.Cerrar()
	})
	r.enviar(paqueteContinue(0, buffer))
	go r.bucle()
	return r, extremoCliente
}

func (r *relayPrueba) bucle() {
	for {
		mensaje, err := r.extremo.Recibir()
		if err != nil {
			return
		}
		paquetes, err := decodificarPaquetes(mensaje)
		if err != nil {
			return
		}
		for _, p := range paquetes {
			r.procesar(p)
		}
	}
}

func (r *relayPrueba) procesar(p paqueteWISP) {
	switch p.tipo {
	case tipoWISPConnect:
		var lado net.Conn
		if r.alConectar != nil {
			lado = r.alConectar(p.flujo)
		}
		f := &flujoRelay{id: p.flujo, lado: lado, entrada: make(chan []byte, 64), cerrado: make(chan struct{})}
		r.mu.Lock()
		r.flujos[p.flujo] = f
		r.mu.Unlock()
		if lado != nil {
			go r.bombearHaciaServidor(f)
			go r.bombearDesdeServidor(f)
		}
	case tipoWISPData:
		r.mu.Lock()
		r.capturado.Write(p.carga)
		r.datos++
		f := r.flujos[p.flujo]
		r.mu.Unlock()
		if r.eco {
			r.enviar(paqueteData(p.flujo, p.carga))
			return
		}
		if f != nil && f.lado != nil {
			copia := make([]byte, len(p.carga))
			copy(copia, p.carga)
			select {
			case f.entrada <- copia:
			case <-f.cerrado:
			}
		}
	case tipoWISPClose:
		r.mu.Lock()
		f := r.flujos[p.flujo]
		r.mu.Unlock()
		if f != nil {
			f.cerrar()
		}
	}
}

func (f *flujoRelay) cerrar() {
	f.unaVez.Do(func() {
		close(f.cerrado)
		close(f.entrada)
		if f.lado != nil {
			_ = f.lado.Close()
		}
	})
}

func (r *relayPrueba) bombearHaciaServidor(f *flujoRelay) {
	for datos := range f.entrada {
		if _, err := f.lado.Write(datos); err != nil {
			return
		}
	}
}

func (r *relayPrueba) bombearDesdeServidor(f *flujoRelay) {
	buf := make([]byte, 32<<10)
	for {
		n, err := f.lado.Read(buf)
		if n > 0 {
			r.enviar(paqueteData(f.id, buf[:n]))
		}
		if err != nil {
			r.enviar(paqueteClose(f.id, motivoCierreVoluntario))
			return
		}
	}
}

func (r *relayPrueba) enviar(b []byte) {
	r.envioMu.Lock()
	defer r.envioMu.Unlock()
	_ = r.extremo.Enviar(b)
}

func (r *relayPrueba) datosRecibidos() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.datos
}

func (r *relayPrueba) capturadoBytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	copia := make([]byte, r.capturado.Len())
	copy(copia, r.capturado.Bytes())
	return copia
}

func esperarDatos(t *testing.T, r *relayPrueba, n int) {
	t.Helper()
	limite := time.Now().Add(2 * time.Second)
	for time.Now().Before(limite) {
		if r.datosRecibidos() >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("el relay recibió %d paquetes DATA, se esperaban al menos %d", r.datosRecibidos(), n)
}

func TestWISPControlDeFlujo(t *testing.T) {
	// El CONTINUE inicial anuncia 2 paquetes, no 128: si el cliente asumiera el
	// valor típico, el tercer Write pasaría y este test fallaría.
	relay, extremo := nuevoRelayPrueba(t, 2, true, nil)
	cliente := NuevoClienteWISP(extremo)
	defer cliente.Cerrar()

	conexion, err := cliente.AbrirTCP("eco.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP: %v", err)
	}
	if _, err := conexion.Write([]byte("a")); err != nil {
		t.Fatalf("primer Write: %v", err)
	}
	if _, err := conexion.Write([]byte("b")); err != nil {
		t.Fatalf("segundo Write: %v", err)
	}
	esperarDatos(t, relay, 2)

	tercero := make(chan error, 1)
	go func() {
		_, err := conexion.Write([]byte("c"))
		tercero <- err
	}()

	select {
	case err := <-tercero:
		t.Fatalf("el tercer Write terminó sin crédito: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if n := relay.datosRecibidos(); n != 2 {
		t.Fatalf("el relay recibió %d DATA mientras el crédito era 0", n)
	}

	relay.enviar(paqueteContinue(1, 5))
	select {
	case err := <-tercero:
		if err != nil {
			t.Fatalf("tercer Write tras CONTINUE: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("el tercer Write no despertó con el CONTINUE")
	}
	esperarDatos(t, relay, 3)
}

func TestWISPCierreDelRelay(t *testing.T) {
	t.Run("motivo del servidor", func(t *testing.T) {
		relay, extremo := nuevoRelayPrueba(t, 8, false, nil)
		cliente := NuevoClienteWISP(extremo)
		defer cliente.Cerrar()

		conexion, err := cliente.AbrirTCP("cerrado.test", 80)
		if err != nil {
			t.Fatalf("AbrirTCP: %v", err)
		}
		relay.enviar(paqueteClose(1, 0x41))
		_, err = conexion.Read(make([]byte, 8))
		if err == nil || !strings.Contains(err.Error(), "0x41") {
			t.Fatalf("Read tras CLOSE = %v, se esperaba el motivo 0x41", err)
		}
	})

	t.Run("cierre voluntario", func(t *testing.T) {
		relay, extremo := nuevoRelayPrueba(t, 8, false, nil)
		cliente := NuevoClienteWISP(extremo)
		defer cliente.Cerrar()

		conexion, err := cliente.AbrirTCP("fin.test", 80)
		if err != nil {
			t.Fatalf("AbrirTCP: %v", err)
		}
		relay.enviar(paqueteClose(1, motivoCierreVoluntario))
		_, err = conexion.Read(make([]byte, 8))
		if !errors.Is(err, io.EOF) {
			t.Fatalf("Read tras CLOSE voluntario = %v, se esperaba io.EOF", err)
		}
	})
}

func TestWISPPlazos(t *testing.T) {
	// Crédito 0: Write tiene que esperar y vencer por plazo, no colgarse.
	relay, extremo := nuevoRelayPrueba(t, 0, false, nil)
	cliente := NuevoClienteWISP(extremo)
	defer cliente.Cerrar()

	conexion, err := cliente.AbrirTCP("lento.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP: %v", err)
	}
	_ = relay

	if err := conexion.SetWriteDeadline(time.Now().Add(30 * time.Millisecond)); err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}
	if _, err := conexion.Write([]byte("x")); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("Write con plazo = %v, se esperaba os.ErrDeadlineExceeded", err)
	}

	if err := conexion.SetReadDeadline(time.Now().Add(30 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	if _, err := conexion.Read(make([]byte, 8)); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("Read con plazo = %v, se esperaba os.ErrDeadlineExceeded", err)
	}
}

func TestWISPClienteCerrado(t *testing.T) {
	_, extremo := nuevoRelayPrueba(t, 8, true, nil)
	cliente := NuevoClienteWISP(extremo)

	conexion, err := cliente.AbrirTCP("cierre.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP: %v", err)
	}
	if err := conexion.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := conexion.Write([]byte("x")); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Write tras Close = %v, se esperaba net.ErrClosed", err)
	}

	cliente.Cerrar()
	if _, err := cliente.AbrirTCP("otro.test", 80); err == nil {
		t.Fatal("AbrirTCP después de Cerrar el cliente debía fallar")
	}
}

func TestWISPFlujosConcurrentes(t *testing.T) {
	const flujos = 8
	const vueltas = 40
	relay, extremo := nuevoRelayPrueba(t, 1<<20, true, nil)
	cliente := NuevoClienteWISP(extremo)
	defer cliente.Cerrar()
	_ = relay

	var wg sync.WaitGroup
	errores := make(chan error, flujos)
	for i := 0; i < flujos; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conexion, err := cliente.AbrirTCP("concurrente.test", uint16(9000+id))
			if err != nil {
				errores <- err
				return
			}
			defer conexion.Close()
			for j := 0; j < vueltas; j++ {
				enviado := []byte{byte(id), byte(j)}
				if _, err := conexion.Write(enviado); err != nil {
					errores <- err
					return
				}
				recibido := make([]byte, len(enviado))
				if _, err := io.ReadFull(conexion, recibido); err != nil {
					errores <- err
					return
				}
				if !bytes.Equal(enviado, recibido) {
					errores <- errors.New("el eco no coincide")
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errores)
	for err := range errores {
		t.Fatalf("flujo concurrente: %v", err)
	}
}
