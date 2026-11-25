package main

// Falsaciones de la contrapresión en los dos sentidos del flujo WISP.

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type contadorBytes struct{ n atomic.Int64 }

func (c *contadorBytes) Write(p []byte) (int, error) {
	c.n.Add(int64(len(p)))
	return len(p), nil
}

func sumideroTCP(t *testing.T) (string, uint16, *contadorBytes) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	contador := &contadorBytes{}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(contador, conn)
			}()
		}
	}()
	return "127.0.0.1", uint16(ln.Addr().(*net.TCPAddr).Port), contador
}

// emisorConPuerta escribe paquetes separados en el socket; la puerta permite
// que el cliente anuncie su crédito antes de que llegue el primer DATA.
func emisorConPuerta(t *testing.T, paquetes int) (string, uint16, chan struct{}, chan struct{}) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	puerta := make(chan struct{})
	salida := make(chan struct{})
	t.Cleanup(func() { close(salida) })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		<-puerta
		for i := 0; i < paquetes; i++ {
			if _, err := conn.Write(bytes.Repeat([]byte{byte(i)}, 512)); err != nil {
				return
			}
			time.Sleep(100 * time.Millisecond) // separar paquetes TCP para que no se coalescen
		}
		<-salida // no cerrar antes de que el cliente termine de leer
	}()
	return "127.0.0.1", uint16(ln.Addr().(*net.TCPAddr).Port), puerta, salida
}

// TestSubidaMayor1MiB imita el Write de apkc/wisp.go:870-916: el cliente no
// puede enviar más DATA cuando su crédito llega a 0 y espera un CONTINUE nuevo.
// Con una ventana de 8 paquetes hacen falta varias renovaciones para 1,25 MiB.
func TestSubidaMayor1MiB(t *testing.T) {
	_, url := servidorPruebaCon(t, politicaPrueba, 8)
	host, puerto, contador := sumideroTCP(t)
	cl := abrirCliente(t, url, origenProduccion)
	inicial := cl.leer()
	credito := creditoDe(inicial)
	if credito == 0 {
		t.Fatal("CONTINUE inicial con crédito 0")
	}

	const id uint32 = 11
	const paquetes = 80 // 80 * 16 KiB = 1,25 MiB
	cl.connect(id, tipoFlujoTCP, host, puerto)
	bloque := bytes.Repeat([]byte{0x5a}, tamMaxDatos)
	for i := 0; i < paquetes; i++ {
		for credito == 0 {
			p := cl.leer()
			if p.tipo == tipoContinue && p.streamID == id {
				credito = creditoDe(p)
				if credito == 0 {
					t.Fatal("el relay anunció CONTINUE con crédito 0")
				}
			}
		}
		cl.enviar(tipoData, id, bloque)
		credito--
	}

	esperado := int64(paquetes * tamMaxDatos)
	plazo := time.Now().Add(5 * time.Second)
	for contador.n.Load() < esperado && time.Now().Before(plazo) {
		time.Sleep(10 * time.Millisecond)
	}
	if recibido := contador.n.Load(); recibido != esperado {
		t.Fatalf("el sumidero recibió %d bytes, quería %d", recibido, esperado)
	}
}

type llegada struct {
	p   paquete
	err error
}

// lectorFondo evita poner un deadline de lectura en el WebSocket: un timeout
// de gorilla deja el conn con un error permanente y no se puede seguir leyendo.
func lectorFondo(ws *websocket.Conn) <-chan llegada {
	ch := make(chan llegada, 16)
	go func() {
		defer close(ch)
		for {
			_, mensaje, err := ws.ReadMessage()
			if err != nil {
				ch <- llegada{err: err}
				return
			}
			p, _, err := decodificarPaquete(mensaje)
			if err != nil {
				ch <- llegada{err: err}
				return
			}
			ch <- llegada{p: p}
		}
	}()
	return ch
}

func esperarPaquete(t *testing.T, ch <-chan llegada) paquete {
	t.Helper()
	select {
	case lleg := <-ch:
		if lleg.err != nil {
			t.Fatalf("leyendo DATA: %v", lleg.err)
		}
		return lleg.p
	case <-time.After(3 * time.Second):
		t.Fatal("no llegó DATA antes del plazo")
		return paquete{}
	}
}

// TestCreditoServidorACliente comprueba la otra dirección: el cliente anuncia
// 4 paquetes, el relay no envía el quinto hasta un CONTINUE nuevo.
func TestCreditoServidorACliente(t *testing.T) {
	_, url := servidorPrueba(t)
	host, puerto, puerta, _ := emisorConPuerta(t, 8)
	cl := abrirCliente(t, url, origenProduccion)
	ch := lectorFondo(cl.ws)
	if p := esperarPaquete(t, ch); p.tipo != tipoContinue || p.streamID != 0 {
		t.Fatalf("CONTINUE inicial: tipo=0x%02x stream=%d", p.tipo, p.streamID)
	}

	const id uint32 = 12
	cl.connect(id, tipoFlujoTCP, host, puerto)
	credito := make([]byte, 4)
	binary.LittleEndian.PutUint32(credito, 4)
	cl.enviar(tipoContinue, id, credito)
	time.Sleep(100 * time.Millisecond) // el relay aplica el CONTINUE antes de recibir datos
	close(puerta)

	for i := 0; i < 4; i++ {
		p := esperarPaquete(t, ch)
		if p.tipo != tipoData || p.streamID != id {
			t.Fatalf("DATA %d: tipo=0x%02x stream=%d", i, p.tipo, p.streamID)
		}
	}

	select {
	case lleg := <-ch:
		if lleg.err == nil && lleg.p.tipo == tipoData && lleg.p.streamID == id {
			t.Fatal("llegó un quinto DATA sin crédito nuevo")
		}
	case <-time.After(250 * time.Millisecond):
	}

	cl.enviar(tipoContinue, id, credito)
	for i := 4; i < 8; i++ {
		p := esperarPaquete(t, ch)
		if p.tipo != tipoData || p.streamID != id {
			t.Fatalf("DATA %d: tipo=0x%02x stream=%d", i, p.tipo, p.streamID)
		}
	}
}
