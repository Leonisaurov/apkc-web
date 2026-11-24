package main

// Pruebas de las cotas de memoria del cliente WISP: bytes por flujo, bytes en
// conjunto, DATA más grande que la cola y crédito de envío. Todas usan el relay
// en memoria, sin red.

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

func colaCerrada(q *colaBytes) bool {
	select {
	case <-q.cerrada:
		return true
	default:
		return false
	}
}

func bytesEnCola(q *colaBytes) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.usado
}

// esperarColaCerrada espera a que el cliente procese el DATA que desborda.
func esperarColaCerrada(t *testing.T, q *colaBytes) {
	t.Helper()
	limite := time.Now().Add(2 * time.Second)
	for time.Now().Before(limite) {
		if colaCerrada(q) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("la cola no se cerró en 2 s")
}

// TestWISPColaPorFlujoAcotada es la sonda del auditor: 2000 DATA de 16 KiB
// (31,25 MiB) sobre un flujo que nadie lee. La cola tiene que cerrarse al llegar
// al tope, no guardarlos todos.
func TestWISPColaPorFlujoAcotada(t *testing.T) {
	relay, extremo := nuevoRelayPrueba(t, 1<<20, false, nil)
	cliente := NuevoClienteWISP(extremo)
	defer cliente.Cerrar()
	conexion, err := cliente.AbrirTCP("lento.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP: %v", err)
	}
	f := conexion.(*flujoWISP)

	const tam = 16 << 10
	const paquetes = 2000
	relleno := bytes.Repeat([]byte{'a'}, tam)
	enviados := 0
	for i := 0; i < paquetes; i++ {
		relay.enviar(paqueteData(f.id, relleno))
		enviados++
		if colaCerrada(f.cola) {
			break
		}
	}

	if !colaCerrada(f.cola) {
		t.Fatalf("la cola no se cerró tras %d paquetes (%d B): no hay cota", enviados, enviados*tam)
	}
	if err := f.comprobarAbierta(); !errors.Is(err, ErrColaDesbordada) {
		t.Fatalf("el flujo quedó con %v, se esperaba ErrColaDesbordada", err)
	}
	enCola := bytesEnCola(f.cola)
	if enCola > topeBytesFlujoWISP {
		t.Fatalf("la cola guardó %d B, por encima del tope de %d B", enCola, topeBytesFlujoWISP)
	}
	if enCola >= 31<<20 {
		t.Fatalf("la cola llegó a %.2f MiB; el tope debía cortar antes de 31 MiB", float64(enCola)/(1<<20))
	}
	if motivo := esperarCierre(t, relay, f.id); motivo != motivoCierreGenerico {
		t.Fatalf("el CLOSE de desborde llevaba el motivo %#x, se esperaba %#x", motivo, motivoCierreGenerico)
	}
	if enUso := cliente.topeGlobal.enUso(); enUso != 0 {
		t.Fatalf("el tope global retiene %d B de un flujo ya cerrado", enUso)
	}

	// Nada más entra: el flujo ya está fuera del mapa y la cola cerrada no crece.
	for i := 0; i < 50; i++ {
		relay.enviar(paqueteData(f.id, relleno))
	}
	time.Sleep(50 * time.Millisecond)
	if crecido := bytesEnCola(f.cola); crecido != enCola {
		t.Fatalf("la cola creció de %d a %d B después del cierre", enCola, crecido)
	}
	t.Logf("cola acotada a %d B (%.2f MiB) con %d paquetes de %d B enviados",
		enCola, float64(enCola)/(1<<20), enviados, tam)
}

// TestWISPColaGlobalAcotada llena un flujo dentro de su tope y comprueba que el
// segundo se cierra al agotarse el tope compartido.
func TestWISPColaGlobalAcotada(t *testing.T) {
	relay, extremo := nuevoRelayPrueba(t, 1<<20, false, nil)
	cliente := NuevoClienteWISP(extremo)
	cliente.topeFlujo = 4 << 20
	cliente.topeGlobal = nuevoTopeBytes(2 << 20)
	defer cliente.Cerrar()

	primera, err := cliente.AbrirTCP("uno.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP (1): %v", err)
	}
	segunda, err := cliente.AbrirTCP("dos.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP (2): %v", err)
	}
	uno := primera.(*flujoWISP)
	dos := segunda.(*flujoWISP)

	// 2 MiB exactos en el primer flujo: cabe en su tope de 4 MiB y agota el global.
	trozo := make([]byte, 64<<10)
	for i := 0; i < 32; i++ {
		relay.enviar(paqueteData(uno.id, trozo))
	}
	limite := time.Now().Add(2 * time.Second)
	for cliente.topeGlobal.enUso() != 2<<20 && time.Now().Before(limite) {
		time.Sleep(2 * time.Millisecond)
	}
	if colaCerrada(uno.cola) {
		t.Fatalf("el primer flujo se cerró con %d B, dentro de su tope de 4 MiB", bytesEnCola(uno.cola))
	}
	if enUso := cliente.topeGlobal.enUso(); enUso != 2<<20 {
		t.Fatalf("el tope global cuenta %d B, se esperaban %d", enUso, 2<<20)
	}

	relay.enviar(paqueteData(dos.id, trozo))
	esperarColaCerrada(t, dos.cola)
	if err := dos.comprobarAbierta(); !errors.Is(err, ErrColaDesbordada) {
		t.Fatalf("el segundo flujo quedó con %v, se esperaba ErrColaDesbordada", err)
	}
	if motivo := esperarCierre(t, relay, dos.id); motivo != motivoCierreGenerico {
		t.Fatalf("el CLOSE global llevaba el motivo %#x", motivo)
	}
	if enUso := cliente.topeGlobal.enUso(); enUso != 2<<20 {
		t.Fatalf("el tope global quedó en %d B tras rechazar el bloque", enUso)
	}
	if _, ok := relay.motivoCierre(uno.id); ok {
		t.Fatal("el primer flujo recibió un CLOSE que no le tocaba")
	}
}

// TestWISPPaqueteMayorQueLaCola manda un único DATA de 2 MiB contra una cola de
// 1 MiB: se rechaza entero y no queda nada retenido.
func TestWISPPaqueteMayorQueLaCola(t *testing.T) {
	relay, extremo := nuevoRelayPrueba(t, 1<<20, false, nil)
	cliente := NuevoClienteWISP(extremo)
	cliente.topeFlujo = 1 << 20
	defer cliente.Cerrar()
	conexion, err := cliente.AbrirTCP("grande.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP: %v", err)
	}
	f := conexion.(*flujoWISP)

	relay.enviar(paqueteData(f.id, make([]byte, 2<<20)))
	esperarColaCerrada(t, f.cola)
	if err := f.comprobarAbierta(); !errors.Is(err, ErrColaDesbordada) {
		t.Fatalf("el flujo quedó con %v, se esperaba ErrColaDesbordada", err)
	}
	if enCola := bytesEnCola(f.cola); enCola != 0 {
		t.Fatalf("el DATA rechazado dejó %d B en la cola", enCola)
	}
	esperarCierre(t, relay, f.id)
}

// TestWISPLecturaParcialNoDuplica lee 1 byte de un DATA de 100 KiB y comprueba
// que el resto sigue en la cola, contabilizado una sola vez.
func TestWISPLecturaParcialNoDuplica(t *testing.T) {
	relay, extremo := nuevoRelayPrueba(t, 1<<20, false, nil)
	cliente := NuevoClienteWISP(extremo)
	defer cliente.Cerrar()
	conexion, err := cliente.AbrirTCP("parcial.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP: %v", err)
	}
	f := conexion.(*flujoWISP)

	datos := make([]byte, 100<<10)
	for i := range datos {
		datos[i] = byte(i)
	}
	relay.enviar(paqueteData(f.id, datos))
	limite := time.Now().Add(2 * time.Second)
	for bytesEnCola(f.cola) == 0 && time.Now().Before(limite) {
		time.Sleep(2 * time.Millisecond)
	}

	primero := make([]byte, 1)
	if n, err := conexion.Read(primero); n != 1 || err != nil {
		t.Fatalf("Read de 1 byte = (%d, %v)", n, err)
	}
	if primero[0] != datos[0] {
		t.Fatalf("el primer byte leído es %#x, se esperaba %#x", primero[0], datos[0])
	}
	if enCola := bytesEnCola(f.cola); enCola != len(datos)-1 {
		t.Fatalf("tras leer 1 byte la cola cuenta %d B, se esperaban %d", enCola, len(datos)-1)
	}
	if enUso := cliente.topeGlobal.enUso(); enUso != len(datos)-1 {
		t.Fatalf("el tope global cuenta %d B, se esperaban %d", enUso, len(datos)-1)
	}

	resto := make([]byte, len(datos)-1)
	if _, err := io.ReadFull(conexion, resto); err != nil {
		t.Fatalf("ReadFull del resto: %v", err)
	}
	if !bytes.Equal(resto, datos[1:]) {
		t.Fatal("el resto leído no coincide con lo enviado")
	}
	if enCola := bytesEnCola(f.cola); enCola != 0 {
		t.Fatalf("la cola quedó con %d B", enCola)
	}
	if enUso := cliente.topeGlobal.enUso(); enUso != 0 {
		t.Fatalf("el tope global quedó con %d B", enUso)
	}
}

// TestWISPCreditoAcotado comprueba que un CONTINUE de 0xFFFFFFFF no deja
// 4.294.967.295 paquetes de crédito y que, agotado el tope, Write espera.
func TestWISPCreditoAcotado(t *testing.T) {
	relay, extremo := nuevoRelayPrueba(t, 0xFFFFFFFF, false, nil)
	cliente := NuevoClienteWISP(extremo)
	defer cliente.Cerrar()
	conexion, err := cliente.AbrirTCP("credito.test", 80)
	if err != nil {
		t.Fatalf("AbrirTCP: %v", err)
	}
	f := conexion.(*flujoWISP)

	if credito := creditoDe(f); credito != creditoMaximoWISP {
		t.Fatalf("el CONTINUE inicial de 0xFFFFFFFF dejó %d paquetes, el tope es %d", credito, creditoMaximoWISP)
	}
	// Gastar unos créditos primero, para que el CONTINUE por flujo se note: si no
	// aplicara el tope, el contador subiría a 4294967295.
	for i := 0; i < 5; i++ {
		if _, err := conexion.Write([]byte{'x'}); err != nil {
			t.Fatalf("Write de calentamiento %d: %v", i, err)
		}
	}
	if credito := creditoDe(f); credito != creditoMaximoWISP-5 {
		t.Fatalf("tras 5 Write el crédito es %d, se esperaban %d", credito, creditoMaximoWISP-5)
	}
	relay.enviar(paqueteContinue(f.id, 0xFFFFFFFF))
	limite := time.Now().Add(2 * time.Second)
	for creditoDe(f) != creditoMaximoWISP && time.Now().Before(limite) {
		time.Sleep(2 * time.Millisecond)
	}
	if credito := creditoDe(f); credito != creditoMaximoWISP {
		t.Fatalf("el CONTINUE por flujo dejó %d paquetes, el tope es %d", credito, creditoMaximoWISP)
	}

	for i := 0; i < creditoMaximoWISP; i++ {
		if _, err := conexion.Write([]byte{'x'}); err != nil {
			t.Fatalf("Write %d con crédito: %v", i, err)
		}
	}
	_ = conexion.SetWriteDeadline(time.Now().Add(50 * time.Millisecond))
	if _, err := conexion.Write([]byte{'x'}); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("Write sin crédito = %v, se esperaba os.ErrDeadlineExceeded", err)
	}
	t.Logf("el crédito quedó topado en %d paquetes tras un CONTINUE de 0xFFFFFFFF", creditoMaximoWISP)
}

func creditoDe(f *flujoWISP) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.credito
}
